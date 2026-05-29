# AgentVault Desktop App Design

## Overview
A local-first desktop knowledge base application built with Wails (Go backend + React frontend). The app provides a fast, keyboard-first interface for managing Markdown notes with AI-powered search and retrieval.

## Tech Stack
- **Backend**: Go (Wails v2 runtime)
- **Frontend**: React 19 + TypeScript + Tailwind CSS v3
- **Editor**: CodeMirror 6 (@codemirror/lang-markdown)
- **State**: React Context (no external state library needed)

## Architecture
The Go backend exposes methods via Wails bindings. The frontend calls these methods through `window.go` runtime.

### Go Backend Methods
```
VaultService:
  - GetVaultPath() string
  - SetVaultPath(path string) error
  - IsVault(path string) bool
  - InitVault(path string) error
  - GetStatus() VaultStatus

NoteService:
  - Search(query string, type?: string, project?: string) SearchResult[]
  - GetNote(id string) Note
  - GetNoteContent(path string) string
  - SaveNote(path string, content string) error
  - CreateNote(type string, title string, project?: string) string
  - GetRecent(limit int) Note[]
  - GetProjects() string[]

IndexService:
  - Index(force bool) IndexResult
  - GetIndexingStatus() IndexingStatus

AIService:
  - Ask(question string) Answer
  - IsAIEnabled() bool
```

## Layout
```
+------------------+--------------------------+------------------+
|                  |                          |                  |
|   Sidebar        |    Main Content          |   Right Panel    |
|   (200px)        |    (flex)                |   (350px,       |
|                  |                          |    collapsible)  |
| - Vault name     |    - Editor / Preview    |                  |
| - File tree      |    - Search results      | - AI Ask panel   |
| - Navigation     |    - Dashboards          | - Note info      |
|   (icons)        |                          | - Git status     |
|                  |                          |                  |
+------------------+--------------------------+------------------+
```

## Color Palette (Dark Mode Default)
```
--bg-primary:     #0f1117
--bg-secondary:   #1a1d27
--bg-tertiary:    #232734
--bg-hover:       #2a2e3b
--border:         #2e3344
--text-primary:   #e4e6eb
--text-secondary: #9ca3af
--text-muted:     #6b7280
--accent:         #4f7cff
--accent-hover:   #6b93ff
--success:        #22c55e
--warning:        #eab308
--error:          #ef4444
```

## Views

### 1. Vault Picker (First Launch)
- Centered modal overlay
- "Open AgentVault" title
- Recent vaults list (clickable)
- "Open Folder" button (opens file picker)
- "Create New Vault" button

### 2. Main Layout (Persistent)
- Left sidebar: vault name, file tree, nav icons
- Center: content area (editor/search/dashboard)
- Right: collapsible AI panel

### 3. Editor View
- Split pane: editor left (60%), preview right (40%)
- Editor: CodeMirror 6 with markdown highlighting
- Preview: rendered markdown HTML
- Toolbar: save status, word count, note type badge

### 4. Search View
- Search bar at top (prominent, keyboard shortcut /)
- Results list with title, type badge, path, excerpt
- Filter pills: type, project, tag
- Keyboard navigation: arrow keys to navigate, enter to open

### 5. AI Ask Panel (Right sidebar)
- Question input at bottom
- Chat history above (user questions + AI answers)
- Answer shows: text + sources list + confidence badge
- Sources clickable → open note

### 6. Dashboards
- Project dashboard: project list with note counts
- Decision dashboard: recent decisions by status
- Source dashboard: sources grouped by type

### 7. Settings
- AI provider config (Ollama URL, model)
- Theme toggle (dark/light)
- Keyboard shortcuts reference
- Vault info (path, note count, index status)

## Keyboard Shortcuts
```
Cmd/Ctrl + K     Command palette / search
Cmd/Ctrl + /     Focus search bar
Cmd/Ctrl + N     New note
Cmd/Ctrl + S     Save note
Cmd/Ctrl + B     Toggle sidebar
Cmd/Ctrl + J     Toggle AI panel
Escape           Close modal / panel
```

## Component Hierarchy
```
App.tsx
├── VaultPicker.tsx          (first-launch modal)
├── Layout.tsx
│   ├── Sidebar.tsx
│   │   ├── VaultHeader.tsx
│   │   ├── FileTree.tsx
│   │   └── NavIcons.tsx
│   ├── MainContent.tsx
│   │   ├── EditorView.tsx
│   │   │   ├── Toolbar.tsx
│   │   │   ├── MarkdownEditor.tsx (CodeMirror)
│   │   │   └── PreviewPane.tsx
│   │   ├── SearchView.tsx
│   │   │   ├── SearchBar.tsx
│   │   │   ├── FilterPills.tsx
│   │   │   └── SearchResults.tsx
│   │   ├── ProjectDashboard.tsx
│   │   ├── DecisionDashboard.tsx
│   │   └── SettingsView.tsx
│   └── AIPanel.tsx
│       ├── ChatHistory.tsx
│       ├── AskInput.tsx
│       └── SourceList.tsx
```

## File Structure
```
apps/desktop-wails/
  main.go                      # Wails entry
  app.go                       # App struct + services
  go.mod
  wails.json
  frontend/
    package.json
    tsconfig.json
    vite.config.ts
    index.html
    src/
      main.tsx
      App.tsx
      wails.d.ts               # Type declarations for window.go
      services/
        vault.ts               # Go binding wrappers
        notes.ts
        search.ts
        ai.ts
      hooks/
        useVault.ts            # React hooks
        useNotes.ts
        useSearch.ts
        useAI.ts
        useKeybinding.ts
      components/
        Layout.tsx
        Sidebar.tsx
        FileTree.tsx
        VaultHeader.tsx
        NavIcons.tsx
        MainContent.tsx
        EditorView.tsx
        MarkdownEditor.tsx     # CodeMirror wrapper
        PreviewPane.tsx
        Toolbar.tsx
        SearchView.tsx
        SearchBar.tsx
        SearchResults.tsx
        FilterPills.tsx
        AIPanel.tsx
        ChatHistory.tsx
        AskInput.tsx
        SourceList.tsx
        VaultPicker.tsx
        ProjectDashboard.tsx
        DecisionDashboard.tsx
        SettingsView.tsx
        CommandPalette.tsx
      types/
        index.ts               # Shared TypeScript types
      styles/
        index.css              # Tailwind + custom variables
```
