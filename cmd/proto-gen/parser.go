package main

import (
	"cmp"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const (
	grpcTag    = "@grpc"
	noRestFlag = "no-rest"
)

type methodInfo struct {
	Name         string // SRPC method name, which is also the gRPC name.
	RequestType  string
	ResponseType string
	NoRest       bool // Not to be exposed over REST.
}

// findMethods returns the SRPC methods in the Go package in dir which carry an
// @grpc annotation, sorted by SRPC method name. The annotation may be written
// "@grpc no-rest" to keep the method off REST. An error is returned if an
// annotated method is not a unary SRPC method.
func findMethods(dir string) ([]methodInfo, error) {
	files, err := parseDir(dir)
	if err != nil {
		return nil, err
	}
	var methods []methodInfo
	for _, file := range files {
		for _, decl := range file.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Recv == nil {
				continue
			}
			tagged, noRest, err := parseGrpcTag(funcDecl.Doc)
			if err != nil {
				return nil, fmt.Errorf("method %s: %w", funcDecl.Name.Name, err)
			}
			if !tagged {
				continue
			}
			method, err := makeMethodInfo(funcDecl, noRest)
			if err != nil {
				return nil, fmt.Errorf("method %s: %w", funcDecl.Name.Name, err)
			}
			methods = append(methods, method)
		}
	}
	slices.SortFunc(methods, func(a, b methodInfo) int {
		return cmp.Compare(a.Name, b.Name)
	})
	return methods, nil
}

// rootTypeNames returns the names of the request and reply types of methods.
func rootTypeNames(methods []methodInfo) []string {
	var names []string
	for _, method := range methods {
		names = append(names, method.RequestType, method.ResponseType)
	}
	return names
}

// parseDir parses the non-test Go files in dir. Build constraints are ignored.
// It returns the parsed files in file name order.
func parseDir(dir string) ([]*ast.File, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	fileSet := token.NewFileSet()
	var files []*ast.File
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, filepath.Join(dir, name), nil,
			parser.ParseComments)
		if err != nil {
			return nil, err
		}
		files = append(files, file)
	}
	return files, nil
}

// parseGrpcTag looks for an @grpc annotation in the doc comment. It returns
// whether one was found and whether it carries the no-rest flag. An error is
// returned for any other argument.
func parseGrpcTag(doc *ast.CommentGroup) (bool, bool, error) {
	if doc == nil {
		return false, false, nil
	}
	for _, comment := range doc.List {
		fields := strings.Fields(strings.TrimPrefix(comment.Text, "//"))
		if len(fields) < 1 || fields[0] != grpcTag {
			continue
		}
		switch {
		case len(fields) == 1:
			return true, false, nil
		case len(fields) == 2 && fields[1] == noRestFlag:
			return true, true, nil
		}
		return false, false, fmt.Errorf("%s takes no argument but %s",
			grpcTag, noRestFlag)
	}
	return false, false, nil
}

// makeMethodInfo returns the description of the unary SRPC method declared by
// funcDecl, which must have the signature
// (conn *srpc.Conn, request T, reply *U) error.
func makeMethodInfo(funcDecl *ast.FuncDecl, noRest bool) (methodInfo, error) {
	method := methodInfo{Name: funcDecl.Name.Name, NoRest: noRest}
	params := funcDecl.Type.Params.List
	if len(params) != 3 {
		return methodInfo{}, fmt.Errorf(
			"want (conn, request, reply) parameters, streaming not supported")
	}
	var err error
	if method.RequestType, err = typeName(params[1].Type); err != nil {
		return methodInfo{}, fmt.Errorf("request: %w", err)
	}
	if method.ResponseType, err = typeName(params[2].Type); err != nil {
		return methodInfo{}, fmt.Errorf("reply: %w", err)
	}
	return method, nil
}

// typeName returns the name of the type of a request or reply parameter,
// without any pointer or package qualifier.
func typeName(expr ast.Expr) (string, error) {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	switch expr := expr.(type) {
	case *ast.Ident:
		return expr.Name, nil
	case *ast.SelectorExpr:
		return expr.Sel.Name, nil
	}
	return "", fmt.Errorf("unsupported type %T", expr)
}
