# Antigravity

## MODIFIED Requirements

### Requirement: Antigravity shares the Gemini global prompt surface
The system MUST preserve Antigravity's `.gemini` and `GEMINI.md` conventions without requiring Gemini CLI selection.
(Previously: warning required simultaneous `gemini-cli` and `antigravity` selection.)

#### Scenario: Prompt surface
- GIVEN Antigravity is selected
- WHEN prompt content installs
- THEN it uses `~/.gemini/GEMINI.md` and Gemini CLI is not exposed.
