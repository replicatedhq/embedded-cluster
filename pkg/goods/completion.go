package goods

import (
	"bytes"
	_ "embed"
	"text/template"
)

var (
	//go:embed static/compalias_bash.tmpl.sh
	compaliasBashText string
)

var (
	compaliasBashTmpl = template.Must(template.New("compalias_bash").Parse(string(compaliasBashText)))
)

// completionAliasBash generates bash completion for the given alias and command.
func completionAliasBash(alias, command string) ([]byte, error) {
	buf := bytes.NewBuffer(nil)
	err := compaliasBashTmpl.Execute(buf, struct {
		Alias   string
		Command string
	}{
		Alias:   alias,
		Command: command,
	})
	return buf.Bytes(), err
}
