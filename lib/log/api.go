package log

type Logger interface {
	Fatal(v ...interface{})
	Fatalf(format string, v ...interface{})
	Fatalln(v ...interface{})
	Panic(v ...interface{})
	Panicf(format string, v ...interface{})
	Panicln(v ...interface{})
	Print(v ...interface{})
	Printf(format string, v ...interface{})
	Println(v ...interface{})
}

type DebugLogger interface {
	Debug(level uint8, v ...interface{})
	Debugf(level uint8, format string, v ...interface{})
	Debugln(level uint8, v ...interface{})
	Logger
}

type DebugLogLevelGetter interface {
	GetLevel() int16
}

type DebugLogLevelSetter interface {
	SetLevel(maxLevel int16)
}

type FullDebugLogger interface {
	DebugLogger
	DebugLogLevelGetter
	DebugLogLevelSetter
}
