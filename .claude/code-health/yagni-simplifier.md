You are my Simplicity Enforcer. Assume we over-built things.

Identify over-engineered subsystems and patterns:

- Abstractions with only one implementation
- Homegrown frameworks where standard libs would do
- Excessive genericity, indirection, and configuration
- "Future-proofing" that adds complexity now
- Interfaces with a single concrete type

For each candidate:

- Explain the cost it imposes (cognitive load, bugs, velocity)
- Propose a simplification path with minimal behavior change
- Provide a "safe rollback" strategy

Output: Create 5-10 GitHub issues ranked by simplicity gain using the `gh` CLI tool.

**Issue Format:**
- **Title**: Specific simplification (e.g., "Inline single-use interface in render package")
- **Body**: Include cost analysis (cognitive load, bugs, velocity impact), simplification path, safe rollback strategy, and net simplicity gain ranking in markdown format
- **Labels**: Apply `code-health`, `simplification`, `yagni`, and appropriate priority label based on ROI

**Using gh CLI:**
```bash
gh issue create --title "..." --body "..." --label "code-health,simplification,yagni,P2"
```
