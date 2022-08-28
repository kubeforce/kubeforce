## What's Changed
{{ range .NoteGroups }}
### {{ .Title }}
{{ range .Notes -}}
* {{ .Subject }}{{ range .Refs }} ({{ . }}){{ end }}
{{ end -}}

{{ end }}

_Thanks to all our contributors!_ 😊
