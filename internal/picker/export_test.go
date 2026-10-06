package picker

import tea "github.com/charmbracelet/bubbletea"

// WaitRefresh blocks until the index refresh of a picker from New has
// written the cache, so nothing writes into a test's temp dirs after they go.
func WaitRefresh(m tea.Model) { <-m.(model).refresh.done }
