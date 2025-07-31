{{ range .Versions }}
<a name="{{ .Tag.Name }}"></a>

## {{ if .Tag.Previous }}[{{ .Tag.Name }}]({{ $.Info.RepositoryURL }}/compare/{{ .Tag.Previous.Name }}...{{ .Tag.Name }}){{ else }}{{ .Tag.Name }}{{ end }}

> {{ datetime "2006-01-02" .Tag.Date }}

{{ range .CommitGroups -}}

### {{ .Title }}

{{ range .Commits -}}
* {{ if .Scope }}**{{ .Scope }}:** {{ end }}{{ .Subject }} ([{{ .Hash.Short }}]({{ $.Info.RepositoryURL }}/commit/{{ .Hash.Long }})){{- if .Author.Email }}{{- if contains .Author.Email "@users.noreply.github.com" }} by @{{ regexReplaceAll "^\\d+\\+(.+)@users\\.noreply\\.github\\.com$" .Author.Email "${1}" }}{{- else if eq .Author.Name "solanyn" }} by @solanyn{{- end }}{{- end }}
{{ end }}

{{ end -}}
{{- if .RevertCommits -}}

### Reverts

{{ range .RevertCommits -}}
* {{ .Revert.Header }}
{{ end }}

{{ end -}}
{{- if .MergeCommits -}}

### Pull Requests

{{ range .MergeCommits -}}
* {{ .Header }}
{{ end }}

{{ end -}}
{{- if .NoteGroups -}}
{{ range .NoteGroups -}}

### Breaking Changes

{{ range .Notes }}
{{ .Body }}
{{ end }}

{{ end -}}
{{ end -}}
{{ end -}}
