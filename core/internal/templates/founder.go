package templates

// StarterTemplate defines a complete vault template.
type StarterTemplate struct {
	Name        string
	Description string
	Folders     []string
	Files       map[string]string
}

var starterRegistry = map[string]*StarterTemplate{}

func RegisterStarter(t *StarterTemplate) {
	starterRegistry[t.Name] = t
}

func GetStarterTemplate(name string) (*StarterTemplate, bool) {
	t, ok := starterRegistry[name]
	return t, ok
}

func ListStarterTemplates() []*StarterTemplate {
	list := make([]*StarterTemplate, 0, len(starterRegistry))
	for _, t := range starterRegistry {
		list = append(list, t)
	}
	return list
}

func init() {
	RegisterStarter(&FounderOSTemplate)
	RegisterStarter(&DeveloperOSTemplate)
	RegisterStarter(&AgentMemoryTemplate)
	RegisterStarter(&ResearchVaultTemplate)
}
