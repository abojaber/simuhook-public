---
title: "Key Bindings"
type: docs
weight: 2
---

# Key Bindings

Press `?` inside the dashboard for this reference anytime.

## Actions

| Key | Action |
|-----|--------|
| `q` / `ctrl+c` | Quit (in the filter prompt, cancels the prompt first) |

## Navigation

The feed has a highlight **cursor**. Cursor keys move it entry-by-entry; the view auto-scrolls to keep the selection visible.

| Key | Action |
|-----|--------|
| `j` / `↓` | Move cursor down one entry (+ auto-scroll) |
| `k` / `↑` | Move cursor up one entry (+ auto-scroll) |
| `pgup` / `pgdn` / `b` / `space` | Jump cursor a full page (+ auto-scroll) |
| `u` / `ctrl+u` / `d` / `ctrl+d` | Jump cursor half a page (+ auto-scroll) |
| `g` / `G` | Jump cursor to the top / bottom of the feed (`g` pauses live-follow, `G` resumes it) |

## Inspecting

| Key | Action |
|-----|--------|
| `enter` | Expand / collapse the selected row inline (full request + response detail) |
| `i` | Toggle the standalone request/response inspector |
| `esc` / `c` | Collapse an expansion / clear a filter (context dependent) |

## Filtering

| Key | Action |
|-----|--------|
| `/` or `f` | Open the filter prompt |
| `enter` | Apply the filter (in prompt) |
| `esc` | Cancel the prompt / clear the filter |
| `c` | Clear the filter |

## Dashboard Control

| Key | Action |
|-----|--------|
| `1` | Switch to main dashboard |
| `2` | Switch to proxy view |
| `L` | Pause / resume live-follow (freezes / resumes the feed) |
| `e` | Toggle the endpoints panel (wide layout) |
| `r` | Manual refresh |
| `?` | Toggle the help overlay |
| `shift+↑` / `shift+↓` or `K` / `J` | Scroll the endpoints panel (wide layout, left visible) |

## Notes

- Cursor movement **never** resumes live-follow — even when the cursor is on the newest entry. Use `L` to resume, or `G` to jump to the bottom and resume.
- Escaping or collapsing an expansion returns the cursor to the newest entry and resumes live-follow.