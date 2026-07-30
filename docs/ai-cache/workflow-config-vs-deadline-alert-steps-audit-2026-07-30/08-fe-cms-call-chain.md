# 08 — FE CMS call chain

TemplateEditorScreen → WorkflowConfigurationPage → useWorkflowConfiguration.getConfiguration
→ GET configuration → render builder/overview from `data.workflow.steps` (no client reorder).
Empty state when no global versions; portal effective count shown separately in editor chrome.
