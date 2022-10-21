package coordinatortest

import (
	"io"
	"net"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/xerrors"

	"github.com/coder/coder/tailnet"
	"github.com/coder/coder/testutil"
)

// CoordinatorFactory creates interconnected coordinators.
type CoordinatorFactory interface {
	New(t testing.TB) tailnet.Coordinator
}

// NewLocalFactory makes a factory that returns the same coordinator each time.
func NewLocalFactory(c tailnet.Coordinator) CoordinatorFactory {
	return &localFactory{coordinator: c}
}

type localFactory struct {
	coordinator tailnet.Coordinator
}

func (l *localFactory) New(testing.TB) tailnet.Coordinator {
	return l.coordinator
}

func RunCoordinatorSuite(t *testing.T,
	newFactory func(testing.TB) CoordinatorFactory,
) {
	t.Run("ClientWithoutAgent", func(t *testing.T) {
		t.Parallel()

		factory := newFactory(t)
		coordinator1 := factory.New(t)
		defer coordinator1.Close()
		coordinator2 := factory.New(t)
		defer coordinator2.Close()

		client, server := net.Pipe()
		sendNode, errChan := tailnet.ServeCoordinator(client, func(node []*tailnet.Node) error { return nil })

		id := uuid.New()
		clientChan := serveClient(t, coordinator1, server, id, uuid.New())
		sendNode(&tailnet.Node{})
		waitNodeExists(t, coordinator1, id)

		closeAndWait(t, client, server, errChan, clientChan)
	})

	t.Run("AgentWithoutClient", func(t *testing.T) {
		t.Parallel()

		factory := newFactory(t)
		coordinator1 := factory.New(t)
		defer coordinator1.Close()
		coordinator2 := factory.New(t)
		defer coordinator2.Close()

		client, server := net.Pipe()
		sendNode, errChan := tailnet.ServeCoordinator(client, func(node []*tailnet.Node) error { return nil })

		id := uuid.New()
		agentChan := serveAgent(t, coordinator1, server, id)
		sendNode(&tailnet.Node{})
		waitNodeExists(t, coordinator1, id)
		// Node should be propagated to both coordinators.
		waitNodeExists(t, coordinator2, id)

		closeAndWait(t, client, server, errChan, agentChan)
	})

	t.Run("AgentBeforeClient", func(t *testing.T) {
		t.Parallel()

		factory := newFactory(t)
		coordinator1 := factory.New(t)
		defer coordinator1.Close()
		coordinator2 := factory.New(t)
		defer coordinator2.Close()

		agentWS, agentServerWS := net.Pipe()
		clientWS, clientServerWS := net.Pipe()

		// Setup agent
		agentID := uuid.New()
		sendAgentNode, agentErrChan, agentNodeChan := serveCoordinator(agentWS)
		closeAgentChan := serveAgent(t, coordinator1, agentServerWS, agentID)
		sendAgentNode(&tailnet.Node{})
		waitNodeExists(t, coordinator1, agentID)
		waitNodeExists(t, coordinator2, agentID)

		// Setup client
		clientID := uuid.New()
		sendClientNode, clientErrChan, clientNodeChan := serveCoordinator(clientWS)
		closeClientChan := serveClient(t, coordinator2, clientServerWS, clientID, agentID)
		sendClientNode(&tailnet.Node{})
		waitNodeExists(t, coordinator2, clientID)

		// Client should immediately get sent the agent node.
		require.Len(t, <-clientNodeChan, 1)
		// Agent should receive the client node.
		require.Len(t, <-agentNodeChan, 1)

		// An update to the agent should make it to the client.
		sendAgentNode(&tailnet.Node{})
		require.Len(t, <-clientNodeChan, 1)

		closeAndWait(t, agentWS, agentServerWS, agentErrChan, closeAgentChan)
		closeAndWait(t, clientWS, clientServerWS, clientErrChan, closeClientChan)
	})

	t.Run("ClientBeforeAgent", func(t *testing.T) {
		t.Parallel()

		factory := newFactory(t)
		coordinator1 := factory.New(t)
		defer coordinator1.Close()
		coordinator2 := factory.New(t)
		defer coordinator2.Close()

		clientWS, clientServerWS := net.Pipe()
		agentWS, agentServerWS := net.Pipe()

		// Setup client
		clientID := uuid.New()
		agentID := uuid.New()
		sendClientNode, clientErrChan, clientNodeChan := serveCoordinator(clientWS)
		closeClientChan := serveClient(t, coordinator1, clientServerWS, clientID, agentID)
		sendClientNode(&tailnet.Node{})
		waitNodeExists(t, coordinator1, clientID)

		// Setup agent
		sendAgentNode, agentErrChan, agentNodeChan := serveCoordinator(agentWS)
		closeAgentChan := serveAgent(t, coordinator2, agentServerWS, agentID)
		sendAgentNode(&tailnet.Node{})
		waitNodeExists(t, coordinator2, agentID)
		waitNodeExists(t, coordinator1, agentID)

		// Client should immediately get sent the agent node.
		require.Len(t, <-clientNodeChan, 1)
		// Agent should receive the client node.
		require.Len(t, <-agentNodeChan, 1)

		// An update to the agent should make it to the client.
		sendAgentNode(&tailnet.Node{})
		require.Len(t, <-clientNodeChan, 1)

		closeAndWait(t, agentWS, agentServerWS, agentErrChan, closeAgentChan)
		closeAndWait(t, clientWS, clientServerWS, clientErrChan, closeClientChan)
	})

	t.Run("AgentReconnect", func(t *testing.T) {
		t.Parallel()

		factory := newFactory(t)
		coordinator1 := factory.New(t)
		defer coordinator1.Close()
		coordinator2 := factory.New(t)
		defer coordinator2.Close()

		agentWS, agentServerWS := net.Pipe()
		clientWS, clientServerWS := net.Pipe()

		// Setup agent
		agentID := uuid.New()
		sendAgentNode, agentErrChan, agentNodeChan := serveCoordinator(agentWS)
		closeAgentChan := serveAgent(t, coordinator1, agentServerWS, agentID)
		sendAgentNode(&tailnet.Node{})
		waitNodeExists(t, coordinator1, agentID)
		waitNodeExists(t, coordinator2, agentID)

		// Setup client
		clientID := uuid.New()
		sendClientNode, clientErrChan, clientNodeChan := serveCoordinator(clientWS)
		closeClientChan := serveClient(t, coordinator2, clientServerWS, clientID, agentID)
		sendClientNode(&tailnet.Node{})
		waitNodeExists(t, coordinator2, clientID)

		// Client should immediately get sent the agent node.
		require.Len(t, <-clientNodeChan, 1)
		// Agent should receive the client node.
		require.Len(t, <-agentNodeChan, 1)

		// An update to the agent should make it to the client.
		sendAgentNode(&tailnet.Node{})
		require.Len(t, <-clientNodeChan, 1)

		// Simulate the agent reconnecting.
		closeAndWait(t, agentWS, agentServerWS, agentErrChan, closeAgentChan)
		agentWS, agentServerWS = net.Pipe()

		// Setup agent
		sendAgentNode, agentErrChan, agentNodeChan = serveCoordinator(agentWS)
		closeAgentChan = serveAgent(t, coordinator1, agentServerWS, agentID)
		sendAgentNode(&tailnet.Node{})

		// Agent should receive the client node upon reconnect.
		require.Len(t, <-agentNodeChan, 1)
		// Client should receive the new agent node.
		require.Len(t, <-clientNodeChan, 1)

		closeAndWait(t, agentWS, agentServerWS, agentErrChan, closeAgentChan)
		closeAndWait(t, clientWS, clientServerWS, clientErrChan, closeClientChan)
	})

	t.Run("ClientReconnect", func(t *testing.T) {
		t.Parallel()

		factory := newFactory(t)
		coordinator1 := factory.New(t)
		defer coordinator1.Close()
		coordinator2 := factory.New(t)
		defer coordinator2.Close()

		agentWS, agentServerWS := net.Pipe()
		clientWS, clientServerWS := net.Pipe()

		// Setup agent
		agentID := uuid.New()
		sendAgentNode, agentErrChan, agentNodeChan := serveCoordinator(agentWS)
		closeAgentChan := serveAgent(t, coordinator1, agentServerWS, agentID)
		sendAgentNode(&tailnet.Node{})
		waitNodeExists(t, coordinator1, agentID)
		waitNodeExists(t, coordinator2, agentID)

		// Setup client
		clientID := uuid.New()
		sendClientNode, clientErrChan, clientNodeChan := serveCoordinator(clientWS)
		closeClientChan := serveClient(t, coordinator2, clientServerWS, clientID, agentID)
		sendClientNode(&tailnet.Node{})
		waitNodeExists(t, coordinator2, clientID)

		// Client should immediately get sent the agent node.
		require.Len(t, <-clientNodeChan, 1)
		// Agent should receive the client node.
		require.Len(t, <-agentNodeChan, 1)

		// An update to the agent should make it to the client.
		sendAgentNode(&tailnet.Node{})
		require.Len(t, <-clientNodeChan, 1)

		// Simulate the client reconnecting.
		closeAndWait(t, clientWS, clientServerWS, clientErrChan, closeClientChan)
		clientWS, clientServerWS = net.Pipe()

		// Setup client
		sendClientNode, clientErrChan, clientNodeChan = serveCoordinator(clientWS)
		closeClientChan = serveClient(t, coordinator2, clientServerWS, clientID, agentID)
		sendClientNode(&tailnet.Node{})

		// Client should receive the agent node.
		require.Len(t, <-clientNodeChan, 1)
		// Agent should receive the client node upon reconnect.
		require.Len(t, <-agentNodeChan, 1)

		closeAndWait(t, agentWS, agentServerWS, agentErrChan, closeAgentChan)
		closeAndWait(t, clientWS, clientServerWS, clientErrChan, closeClientChan)
	})
}

func closeAndWait(t testing.TB, conn1, conn2 net.Conn, errCh <-chan error, closeCh <-chan struct{}) {
	require.NoError(t, conn1.Close())
	require.NoError(t, conn2.Close())
	<-errCh
	<-closeCh
}

func serveCoordinator(conn net.Conn) (func(node *tailnet.Node), <-chan error, <-chan []*tailnet.Node) {
	nodeChan := make(chan []*tailnet.Node)
	sendNode, errChan := tailnet.ServeCoordinator(conn, func(nodes []*tailnet.Node) error {
		nodeChan <- nodes
		return nil
	})

	return sendNode, errChan, nodeChan
}

func waitNodeExists(t testing.TB, coordinator tailnet.Coordinator, id uuid.UUID) {
	t.Helper()
	require.Eventually(t, func() bool {
		return coordinator.Node(id) != nil
	}, testutil.WaitShort, testutil.IntervalFast)
}

func serveClient(t testing.TB, coordinator tailnet.Coordinator, conn net.Conn, id, agent uuid.UUID) <-chan struct{} {
	closeChan := make(chan struct{})
	go func() {
		defer close(closeChan)
		err := coordinator.ServeClient(conn, id, agent)
		if xerrors.Is(err, io.ErrClosedPipe) {
			err = nil
		}
		assert.NoError(t, err)
	}()
	return closeChan
}

func serveAgent(t testing.TB, coord tailnet.Coordinator, conn net.Conn, id uuid.UUID) <-chan struct{} {
	closeChan := make(chan struct{})
	go func() {
		defer close(closeChan)
		err := coord.ServeAgent(conn, id)
		if xerrors.Is(err, io.ErrClosedPipe) {
			err = nil
		}
		assert.NoError(t, err)
	}()
	return closeChan
}
