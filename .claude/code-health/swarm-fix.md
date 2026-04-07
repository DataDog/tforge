You are my Code Health Fixer. Pick the top 1-3 highest ROI code-health issues from GitHub and implement them end-to-end.

**Finding Issues:**
```bash
gh issue list --label "code-health" --state open --sort created --limit 20
```

Rules:

- One commit per issue, directly on main
- Commit message must include "Closes #XX" to auto-close the issue
- Add or update tests first if risk warrants it
- Do not refactor adjacent code "because it's there"
- Maintain backward compatibility unless explicitly allowed
- Update docs/comments where behavior or expectations change
- Run `make lint` and `make test` before committing
- Push after each commit

Commit format:
```bash
git commit -m "$(cat <<'EOF'
<description of what was done>

Closes #XX
EOF
)"
git push
```

Output:

- What you changed and why, per issue
- Tests added/updated and what they cover
- Create new GitHub issues for any follow-up tasks discovered during implementation
