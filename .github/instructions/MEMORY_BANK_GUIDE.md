# Memory Bank Guide

This guide explains how to use the Memory Bank system for AI-assisted development sessions.

## What is Memory Bank?

Memory Bank is a documentation system that allows AI assistants to maintain context across sessions. Since AI memory resets between sessions, the Memory Bank serves as persistent storage for:

- Project context and goals
- Architectural decisions
- Current progress
- Active tasks
- Learned patterns

## Directory Structure

```
memory-bank/
├── projectbrief.md      # Foundation document - project goals and scope
├── productContext.md    # Why the project exists, problems it solves
├── activeContext.md     # Current work focus, recent changes
├── systemPatterns.md    # Architecture, patterns, technical decisions
├── techContext.md       # Technologies, setup, dependencies
├── progress.md          # What works, what's left, session history
└── tasks/
    ├── _index.md        # Master list of all tasks
    └── TASK###-name.md  # Individual task files
```

## How to Use

### Starting a Session

At the start of every session, the AI should:
1. Read all memory bank files
2. Understand current context
3. Continue from where the last session left off

You can prompt this with: "Review the memory bank and continue from where we left off."

### During a Session

The AI will automatically suggest memory bank updates when:
- Making architectural decisions
- Completing significant features
- Encountering important patterns

You can also request updates: "Update the memory bank with our progress."

### Key Commands

| Command | Description |
|---------|-------------|
| `update memory bank` | Review and update all memory bank files |
| `create task` | Create a new task in the tasks folder |
| `update task [ID]` | Update an existing task |
| `show tasks [filter]` | Display tasks (all, active, pending, completed) |

## File Purposes

### projectbrief.md
The foundation document. Defines:
- What the project is
- Why it exists
- Core goals
- Success criteria

### productContext.md
Explains the product from a user perspective:
- Problems being solved
- Target users
- User experience goals
- Comparison with alternatives

### activeContext.md
Current session focus:
- What's being worked on
- Recent changes
- Next steps
- Active decisions

### systemPatterns.md
Technical architecture:
- System design
- Key patterns used
- Design decisions
- Component relationships

### techContext.md
Technical environment:
- Tech stack
- Development setup
- Dependencies
- Constraints

### progress.md
Track progress:
- What works
- What's remaining
- Session history
- Known issues

### tasks/_index.md
Master task list with statuses. Quick reference for all tasks.

### tasks/TASK###-name.md
Individual task files with:
- Original request
- Thought process
- Implementation plan
- Progress log

## Best Practices

1. **Keep files updated** - Stale documentation is worse than none
2. **Be specific** - Include concrete details, not vague descriptions
3. **Track decisions** - Document WHY choices were made
4. **Session boundaries** - Update at end of each session
5. **Cross-reference** - Link between related concepts

## Integration with Instructions

The `.github/instructions/` folder contains AI behavior instructions:
- `personal-working-style.instructions.md` - How the AI should work
- `memory-bank.instructions.md` - Memory bank rules

These files configure how the AI uses the memory bank and interacts with you.
