package remotemcp

const SupportedProtocolVersion = "2025-06-18"

func negotiate(offered string) bool { return offered == SupportedProtocolVersion }
