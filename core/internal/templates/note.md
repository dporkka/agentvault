---
id: {{.ID}}
type: note
title: {{.Title}}
status: active
{{if .Project}}project: {{.Project}}
{{end}}{{if .Tags}}tags: [{{join .Tags ", "}}]
{{end}}created: {{.Created}}
updated: {{.Created}}
---

# {{.Title}}

## Notes

## Related

