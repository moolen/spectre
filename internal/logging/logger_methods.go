package logging

import "context"

// shouldLog checks if a log message at the given level should be output.
// Considers both the logger's level and any per-package level overrides.
func (l *Logger) shouldLog(level LogLevel) bool {
	if pkgLevel := GetPackageLogLevel(l.name); pkgLevel >= 0 {
		return level >= pkgLevel
	}

	return level >= l.level
}

// Debug logs a debug message.
func (l *Logger) Debug(msg string, args ...interface{}) {
	if l.shouldLog(DEBUG) {
		l.logf("DEBUG", msg, args...)
	}
}

// Info logs an info message.
func (l *Logger) Info(msg string, args ...interface{}) {
	if l.shouldLog(INFO) {
		l.logf("INFO", msg, args...)
	}
}

// Warn logs a warning message.
func (l *Logger) Warn(msg string, args ...interface{}) {
	if l.shouldLog(WARN) {
		l.logf("WARN", msg, args...)
	}
}

// Error logs an error message.
func (l *Logger) Error(msg string, args ...interface{}) {
	if l.shouldLog(ERROR) {
		l.logf(strError, msg, args...)
	}
}

// Fatal logs a fatal message and exits the program with code 1.
func (l *Logger) Fatal(msg string, args ...interface{}) {
	if l.shouldLog(FATAL) {
		l.logf("FATAL", msg, args...)
		exitFunc(1)
	}
}

// FatalWithFields logs a fatal message with structured fields and exits the program with code 1.
func (l *Logger) FatalWithFields(msg string, fields ...LogField) {
	if l.shouldLog(FATAL) {
		l.logWithFields("FATAL", msg, fields...)
		exitFunc(1)
	}
}

// ErrorWithErr logs an error message with an error object.
func (l *Logger) ErrorWithErr(msg string, err error, args ...interface{}) {
	if l.shouldLog(ERROR) {
		args = append(args, err)
		l.logf("ERROR", msg+" - %v", args...)
	}
}

// WithName returns a new logger with a custom name.
func (l *Logger) WithName(name string) *Logger {
	return &Logger{
		level:  l.level,
		name:   name,
		fields: make(map[string]interface{}),
		ctx:    l.ctx,
	}
}

// WithField adds a structured field to the logger.
func (l *Logger) WithField(key string, value interface{}) *Logger {
	newLogger := &Logger{
		level:  l.level,
		name:   l.name,
		fields: cloneFields(l.fields),
		ctx:    l.ctx,
	}
	newLogger.fields[key] = value
	return newLogger
}

// WithFields adds multiple structured fields to the logger.
func (l *Logger) WithFields(fields ...LogField) *Logger {
	newLogger := &Logger{
		level:  l.level,
		name:   l.name,
		fields: cloneFields(l.fields),
		ctx:    l.ctx,
	}
	for _, f := range fields {
		newLogger.fields[f.Key] = f.Value
	}
	return newLogger
}

// WithContext returns a new logger with the provided context attached.
// The context is used to extract trace_id and span_id values if present.
func (l *Logger) WithContext(ctx context.Context) *Logger {
	return &Logger{
		level:  l.level,
		name:   l.name,
		fields: cloneFields(l.fields),
		ctx:    ctx,
	}
}

// DebugWithFields logs a debug message with structured fields.
func (l *Logger) DebugWithFields(msg string, fields ...LogField) {
	if l.shouldLog(DEBUG) {
		l.logWithFields("DEBUG", msg, fields...)
	}
}

// InfoWithFields logs an info message with structured fields.
func (l *Logger) InfoWithFields(msg string, fields ...LogField) {
	if l.shouldLog(INFO) {
		l.logWithFields("INFO", msg, fields...)
	}
}

// WarnWithFields logs a warning message with structured fields.
func (l *Logger) WarnWithFields(msg string, fields ...LogField) {
	if l.shouldLog(WARN) {
		l.logWithFields("WARN", msg, fields...)
	}
}

// ErrorWithFields logs an error message with structured fields.
func (l *Logger) ErrorWithFields(msg string, fields ...LogField) {
	if l.shouldLog(ERROR) {
		l.logWithFields("ERROR", msg, fields...)
	}
}

// logWithFields logs a message with structured fields.
func (l *Logger) logWithFields(level, msg string, fields ...LogField) {
	contextFields := extractContextFields(l.ctx)

	var mergedFields map[string]interface{}
	if contextFields != nil || len(l.fields) > 0 || len(fields) > 0 {
		mergedFields = make(map[string]interface{})
		for k, v := range contextFields {
			mergedFields[k] = v
		}
		for k, v := range l.fields {
			mergedFields[k] = v
		}
		for _, f := range fields {
			mergedFields[f.Key] = f.Value
		}
	}

	l.writeLog(level, msg, mergedFields)
}
