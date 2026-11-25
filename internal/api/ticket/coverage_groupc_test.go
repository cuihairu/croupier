// Covers the remaining reachable branches of the ticket package (group C):
// service.Transition re-reading the comment list after a successful update
// fails (the ticket_comments table is missing).
package ticket

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestServiceTransition_ListCommentsError(t *testing.T) {
	s, db := newV9TicketService(t, false)
	id := createV9Ticket(t, s)

	// Both the status change and the ticket re-read succeed; only the comment list query fails.
	require.NoError(t, db.Migrator().DropTable("ticket_comments"))
	_, err := s.Transition(context.Background(), &TransitionRequest{ID: id, Status: "closed"})
	require.Error(t, err)
}
