package lint

import (
	"fmt"
	"runtime"
	"strings"

	"github.com/fatih/color"
)

var (
	red  = color.New(color.FgRed).SprintFunc()
	blue = color.New(color.FgBlue).SprintFunc()
)

type LinterMessages []LinterMessage

type LinterMessage struct {
	isError     bool   // error/suggestion
	caller      string // linter func which created the message
	sourceFile  string // file containing path
	path        string // key path, e.g. key1.key2.setting
	message     string // mandatory message
	description string // optional description, suggestion, information
}

func newError(sourceFile, path, message string, arg ...interface{}) LinterMessage {
	return LinterMessage{
		caller:      getCaller(),
		isError:     true,
		sourceFile:  sourceFile,
		path:        path,
		message:     fmt.Sprintf(message, arg...),
		description: "",
	}
}

func newMessage(sourceFile, path, message string, arg ...interface{}) LinterMessage {
	return LinterMessage{
		caller:      getCaller(),
		isError:     false,
		sourceFile:  sourceFile,
		path:        path,
		message:     fmt.Sprintf(message, arg...),
		description: "",
	}
}

func (lm LinterMessage) WithDescription(desc string, arg ...interface{}) LinterMessage {
	lm.description = fmt.Sprintf(desc, arg...)
	return lm
}

func (lm LinterMessage) IsError() bool {
	return lm.isError
}

func (lm LinterMessage) String() string {
	return lm.Message(false, true)
}

func (lm LinterMessage) Message(withCaller, withDescription bool) string {
	outputColor := blue
	if lm.isError {
		outputColor = red
	}

	caller := ""
	if lm.caller != "" && withCaller {
		caller = fmt.Sprintf("[%s] ", lm.caller)
	}

	desc := ""
	if lm.description != "" && withDescription {
		desc = "\n" + strings.Repeat(" ", len(caller)) + lm.description
	}

	return fmt.Sprintf(
		"%s%s: %s %s %s",
		caller,
		outputColor(lm.sourceFile),
		outputColor("."+lm.path),
		lm.message,
		desc,
	)
}

// LinterMessages implements sort.Interface
func (m LinterMessages) Len() int {
	return len(m)
}

func (m LinterMessages) Less(i, j int) bool {
	switch {
	case m[i].isError == m[j].isError:
		switch {
		case m[i].sourceFile == m[j].sourceFile:
			return m[i].path <= m[j].path
		default:
			return m[i].sourceFile <= m[j].sourceFile
		}
	case m[i].isError && !m[j].isError:
		return true
	case !m[i].isError && m[j].isError:
		return false
	default: // this should never happen
		return false
	}
}

func (m LinterMessages) Swap(i, j int) {
	m[i], m[j] = m[j], m[i]
}

func getCaller() string {
	pc, _, _, ok := runtime.Caller(2)
	details := runtime.FuncForPC(pc)
	if ok && details != nil {
		elem := strings.Split(details.Name(), ".")
		return elem[len(elem)-1]
	}
	return ""
}
