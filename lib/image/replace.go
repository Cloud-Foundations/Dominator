package image

func (annotation *Annotation) registerStrings(registerFunc func(string)) {
	if annotation != nil {
		registerFunc(annotation.URL)
	}
}

func (annotation *Annotation) replaceStrings(replaceFunc func(string) string) {
	if annotation != nil {
		annotation.URL = replaceFunc(annotation.URL)
	}
}

func (image *Image) registerStrings(registerFunc func(string)) {
	registerFunc(image.CreatedBy)
	image.Filter.RegisterStrings(registerFunc)
	image.FileSystem.RegisterStrings(registerFunc)
	image.Triggers.RegisterStrings(registerFunc)
	image.ReleaseNotes.registerStrings(registerFunc)
	image.BuildLog.registerStrings(registerFunc)
	for index := range image.Packages {
		pkg := &image.Packages[index]
		pkg.registerStrings(registerFunc)
	}
	for key, value := range image.Tags {
		registerFunc(key)
		registerFunc(value)
	}
}

func (image *Image) replaceStrings(replaceFunc func(string) string) {
	image.CreatedBy = replaceFunc(image.CreatedBy)
	image.Filter.ReplaceStrings(replaceFunc)
	image.FileSystem.ReplaceStrings(replaceFunc)
	image.Triggers.ReplaceStrings(replaceFunc)
	image.ReleaseNotes.replaceStrings(replaceFunc)
	image.BuildLog.replaceStrings(replaceFunc)
	for index := range image.Packages {
		pkg := &image.Packages[index]
		pkg.replaceStrings(replaceFunc)
	}
	for key, value := range image.Tags {
		image.Tags[key] = replaceFunc(value)
	}
}

func (pkg *Package) registerStrings(registerFunc func(string)) {
	registerFunc(pkg.Name)
	registerFunc(pkg.Version)
}

func (pkg *Package) replaceStrings(replaceFunc func(string) string) {
	pkg.Name = replaceFunc(pkg.Name)
	pkg.Version = replaceFunc(pkg.Version)
}
