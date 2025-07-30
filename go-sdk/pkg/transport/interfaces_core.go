package transport

// This file is intentionally left with only the package declaration to avoid
// interface redeclaration. All transport interfaces and their component interfaces
// (Transport, StreamingTransport, ReliableTransport, Connector, Sender, Receiver,
// ConfigProvider, StatsProvider, BatchSender, EventHandlerProvider, StreamController,
// StreamingStatsProvider, ReliableSender, AckHandlerProvider, ReliabilityStatsProvider,
// EventHandler, AckHandler, TransportEvent) are defined in interfaces.go.