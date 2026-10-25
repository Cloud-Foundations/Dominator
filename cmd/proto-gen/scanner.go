package main

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"reflect"
	"slices"
	"strconv"
)

type fieldInfo struct {
	Name   string
	GoType string
	Number int
}

type typeInfo struct {
	Name   string
	Fields []fieldInfo
}

// scanTypes returns the struct types named in rootNames, the struct types in
// the Go package in dir which carry an @grpc annotation, and every struct type
// in the package which they refer to, sorted by name. Types from other
// packages and named types which are not structs are not followed. An error is
// returned if a root type is not a struct declared in dir, an annotation is
// malformed or a field has a type which cannot be expressed in a proto
// message.
func scanTypes(dir string, rootNames []string) ([]typeInfo, error) {
	files, err := parseDir(dir)
	if err != nil {
		return nil, err
	}
	structs := make(map[string]*ast.StructType)
	queue := append([]string(nil), rootNames...)
	for _, file := range files {
		for _, decl := range file.Decls {
			genDecl, ok := decl.(*ast.GenDecl)
			if !ok || genDecl.Tok != token.TYPE {
				continue
			}
			for _, spec := range genDecl.Specs {
				typeSpec := spec.(*ast.TypeSpec)
				structType, ok := typeSpec.Type.(*ast.StructType)
				if !ok {
					continue
				}
				structs[typeSpec.Name.Name] = structType
				tagged, _, err := parseGrpcTag(typeDoc(genDecl, typeSpec))
				if err != nil {
					return nil, fmt.Errorf("type %s: %w",
						typeSpec.Name.Name, err)
				}
				if tagged {
					queue = append(queue, typeSpec.Name.Name)
				}
			}
		}
	}
	for _, name := range rootNames {
		if _, ok := structs[name]; !ok {
			return nil, fmt.Errorf("struct %s not found in %s", name, dir)
		}
	}
	var result []typeInfo
	visited := make(map[string]struct{})
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if _, ok := visited[name]; ok {
			continue
		}
		visited[name] = struct{}{}
		structType, ok := structs[name]
		if !ok {
			continue
		}
		info, err := makeTypeInfo(name, structType)
		if err != nil {
			return nil, fmt.Errorf("type %s: %w", name, err)
		}
		result = append(result, info)
		for _, field := range structType.Fields.List {
			names, err := referencedNames(field.Type)
			if err != nil {
				return nil, fmt.Errorf("type %s: %w", name, err)
			}
			queue = append(queue, names...)
		}
	}
	slices.SortFunc(result, func(a, b typeInfo) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return result, nil
}

// typeDoc returns the doc comment of a type declaration, which is attached to
// the spec in a parenthesised group and to the declaration otherwise.
func typeDoc(genDecl *ast.GenDecl, typeSpec *ast.TypeSpec) *ast.CommentGroup {
	if typeSpec.Doc != nil {
		return typeSpec.Doc
	}
	return genDecl.Doc
}

// makeTypeInfo returns the description of the struct type name. Fields tagged
// proto:"-" and a string field called Error are left out. Fields tagged
// proto:"N" get number N and the rest get the lowest unused numbers, in
// declaration order. An error is returned for an embedded field or a duplicate
// or invalid number.
func makeTypeInfo(name string, structType *ast.StructType) (typeInfo, error) {
	info := typeInfo{Name: name}
	used := make(map[int]string)
	var fields []fieldInfo
	for _, field := range structType.Fields.List {
		if len(field.Names) < 1 {
			return typeInfo{}, fmt.Errorf("embedded field %s not supported",
				types.ExprString(field.Type))
		}
		number, skip, err := parseProtoTag(field.Tag)
		if err != nil {
			return typeInfo{}, fmt.Errorf("field %s: %w",
				field.Names[0].Name, err)
		}
		goType := types.ExprString(field.Type)
		for _, ident := range field.Names {
			if skip || (ident.Name == "Error" && goType == "string") {
				continue
			}
			if number > 0 {
				if other, ok := used[number]; ok {
					return typeInfo{}, fmt.Errorf(
						"fields %s and %s share number %d",
						other, ident.Name, number)
				}
				used[number] = ident.Name
			}
			fields = append(fields,
				fieldInfo{Name: ident.Name, GoType: goType, Number: number})
		}
	}
	next := 1
	for _, field := range fields {
		if field.Number < 1 {
			for used[next] != "" {
				next++
			}
			field.Number = next
			used[next] = field.Name
		}
		info.Fields = append(info.Fields, field)
	}
	return info, nil
}

// parseProtoTag extracts the proto struct tag. It returns the field number,
// which is zero if the field is to be numbered automatically, and whether the
// field is to be skipped (tag "-").
func parseProtoTag(tag *ast.BasicLit) (number int, skip bool, err error) {
	if tag == nil {
		return 0, false, nil
	}
	unquoted, err := strconv.Unquote(tag.Value)
	if err != nil {
		return 0, false, err
	}
	value := reflect.StructTag(unquoted).Get("proto")
	switch value {
	case "":
		return 0, false, nil
	case "-":
		return 0, true, nil
	}
	number, err = strconv.Atoi(value)
	if err != nil || number < 1 {
		return 0, false, fmt.Errorf("invalid proto tag %q", value)
	}
	return number, false, nil
}

// referencedNames returns the names of the types declared in the same package
// which are used by the field type expr. Predeclared types and types qualified
// by a package name are not included. An error is returned if expr is not a
// name, pointer, slice, array or map type, as nothing else can be expressed in
// a proto message.
func referencedNames(expr ast.Expr) ([]string, error) {
	switch expr := expr.(type) {
	case *ast.Ident:
		if types.Universe.Lookup(expr.Name) == nil {
			return []string{expr.Name}, nil
		}
		return nil, nil
	case *ast.SelectorExpr:
		return nil, nil
	case *ast.StarExpr:
		return referencedNames(expr.X)
	case *ast.ArrayType:
		return referencedNames(expr.Elt)
	case *ast.MapType:
		keyNames, err := referencedNames(expr.Key)
		if err != nil {
			return nil, err
		}
		valueNames, err := referencedNames(expr.Value)
		return append(keyNames, valueNames...), err
	}
	return nil, fmt.Errorf("unsupported type %s", types.ExprString(expr))
}
