---
description: Prepare current PR for review by verifying completeness and updating documentation
---

You are preparing a GitHub Pull Request for review. Follow these steps:

1. **Identify PR Context:**
   - Use `gh pr view --json number,title,body,headRefName` to get current PR details
   - Look for related documentation in `docs/` directory that matches the PR scope

2. **Analyze Planned Work:**
   - Read relevant documentation files in `docs/` to understand what was planned
   - If no specific doc exists, infer from PR title and commit messages
   - Use `gh pr view --json commits` to see commit history if needed

3. **Verify Implementation Completeness:**
   - Run `git diff main...HEAD` to see all changes in the PR
   - Compare changes against planned work to identify:
     - ✅ Completed items
     - ❌ Missing implementations
     - ⚠️  Partial implementations
   - Check if CLAUDE.md needs updates (new commands, dependencies, patterns)
   - Check if README.md needs updates (new features, API endpoints, deployment changes)

4. **Update Documentation:**
   - Update CLAUDE.md if new patterns, commands, or dependencies were added
   - Update README.md if user-facing features or deployment process changed
   - Update or create relevant docs in `docs/` directory if needed
   - Ensure all code examples in docs are accurate

5. **Quality Checks:**
   - Verify `make lint` would pass (check for common issues)
   - Ensure proper error handling patterns are used
   - Check if tests were added for new functionality
   - Verify Kubernetes manifests have proper labels and resource limits (if applicable)

6. **Prepare PR Description:**
   - Create a comprehensive PR description with:
     - **Summary:** Brief overview of changes (2-3 sentences)
     - **Changes:** Bulleted list of what was implemented
     - **Testing:** How to test the changes
     - **Documentation:** What docs were updated
     - **Checklist:** Any remaining items or follow-ups
   - Use markdown formatting for clarity

7. **Push and Update PR:**
   - If documentation was updated, stage and commit changes:
     - `git add <files>`
     - `git commit -m "docs: update documentation for PR"`
   - Push changes: `git push`
   - Update PR title and description: `gh pr edit <number> --title "..." --body "..."`

8. **Final Summary:**
   - Provide a summary of:
     - What was completed vs planned
     - What documentation was updated
     - Any gaps or follow-up work needed
     - Link to the PR for easy access

**Important Notes:**
- Be thorough but concise in your analysis
- If something is missing from the implementation, clearly flag it
- Don't make assumptions - verify by reading actual code changes
- Ensure PR description is clear enough for reviewers who aren't familiar with the codebase
- Follow conventional commit style for any documentation commits
