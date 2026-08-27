package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/javi/eigenmemory/internal/config"
	"github.com/javi/eigenmemory/internal/core"
)

// Handler is a function that handles a tool call and returns a result.
type Handler func(ctx context.Context, s *Server, args json.RawMessage) (any, error)

// Server is a minimal MCP server over stdio.
type Server struct {
	reader      *bufio.Reader
	writer      io.Writer
	tools       map[string]Tool
	handlers    map[string]Handler
	resources   map[string]Resource
	getResource ResourceFetcher
	initialized bool
	store       *core.Store
}

// NewServer creates a new MCP server bound to stdin/stdout.
func NewServer(store *core.Store) *Server {
	return &Server{
		reader:    bufio.NewReader(os.Stdin),
		writer:    os.Stdout,
		tools:     make(map[string]Tool),
		handlers:  make(map[string]Handler),
		resources: make(map[string]Resource),
		store:     store,
	}
}

// RegisterTool adds a tool and its handler.
func (s *Server) RegisterTool(tool Tool, handler Handler) {
	s.tools[tool.Name] = tool
	s.handlers[tool.Name] = handler
}

// ResourceFetcher reads a resource given the active server.
type ResourceFetcher func(s *Server, uri string) (ResourceReadResult, error)

// RegisterResource adds a static resource and a fetch function.
func (s *Server) RegisterResource(res Resource, fetch ResourceFetcher) {
	s.resources[res.URI] = res
	if s.getResource == nil {
		s.getResource = fetch
	}
}

// Run starts the JSON-RPC read loop. It blocks until stdin closes.
func (s *Server) Run() error {
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("read stdin: %w", err)
		}

		var req Request
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			if err := s.writeError(nil, newError(ErrParseError, "parse error", err.Error())); err != nil {
				log.Printf("mcp write error response: %v", err)
			}
			continue
		}

		if err := s.handleRequest(req); err != nil {
			log.Printf("mcp request error: %v", err)
		}
	}
}

func (s *Server) handleRequest(req Request) error {
	if req.JSONRPC != "2.0" {
		return s.writeError(req.ID, newError(ErrInvalidRequest, "invalid jsonrpc version", nil))
	}

	// Per the MCP spec, the server must reject requests other than the
	// initialize handshake itself until initialization has completed.
	if req.Method != "initialize" && !s.initialized {
		if isNotification(req.ID) {
			return nil
		}
		return s.writeError(req.ID, newError(ErrServerNotInitialized, "server not initialized", nil))
	}

	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "initialized":
		// Notification, no response.
		return nil
	case "tools/list":
		return s.handleToolsList(req)
	case "tools/call":
		return s.handleToolCall(req)
	case "resources/list":
		return s.handleResourcesList(req)
	case "resources/read":
		return s.handleResourcesRead(req)
	case "$/cancelRequest":
		return nil
	default:
		if isNotification(req.ID) {
			return nil
		}
		return s.writeError(req.ID, newError(ErrMethodNotFound, fmt.Sprintf("method not found: %s", req.Method), nil))
	}
}

// supportedProtocolVersions lists MCP protocol versions this server
// understands, newest first. The first entry is offered to clients that
// request an unrecognized or absent version.
var supportedProtocolVersions = []string{"2025-06-18", "2024-11-05"}

// negotiateProtocolVersion returns the protocol version the server will use
// for this session: the client's requested version if supported, otherwise
// the server's preferred (latest) version.
func negotiateProtocolVersion(requested string) string {
	for _, v := range supportedProtocolVersions {
		if v == requested {
			return v
		}
	}
	return supportedProtocolVersions[0]
}

func (s *Server) handleInitialize(req Request) error {
	var params InitializeParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.writeError(req.ID, newError(ErrInvalidParams, "invalid initialize params", err.Error()))
		}
	}

	result := InitializeResult{
		ProtocolVersion: negotiateProtocolVersion(params.ProtocolVersion),
		Capabilities: ServerCapabilities{
			Tools:     &ToolsCapability{},
			Resources: &ResourcesCapability{},
		},
		ServerInfo: ServerInfo{
			Name:    "eigenmemory",
			Version: "0.1.0",
		},
	}

	s.initialized = true
	return s.writeResult(req.ID, result)
}

func (s *Server) handleToolsList(req Request) error {
	var list []Tool
	for _, t := range s.tools {
		list = append(list, t)
	}
	return s.writeResult(req.ID, ToolsListResult{Tools: list})
}

func (s *Server) handleToolCall(req Request) error {
	var params ToolCallParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.writeError(req.ID, newError(ErrInvalidParams, "invalid tool call params", err.Error()))
		}
	}

	handler, ok := s.handlers[params.Name]
	if !ok {
		return s.writeError(req.ID, newError(ErrMethodNotFound, fmt.Sprintf("tool not found: %s", params.Name), nil))
	}

	result, err := handler(context.Background(), s, params.Arguments)
	if err != nil {
		return s.writeResult(req.ID, ToolCallResult{
			Content: []ContentItem{{Type: "text", Text: err.Error()}},
			IsError: true,
		})
	}

	return s.writeResult(req.ID, result)
}

func (s *Server) handleResourcesList(req Request) error {
	var list []Resource
	for _, r := range s.resources {
		list = append(list, r)
	}
	return s.writeResult(req.ID, ResourcesListResult{Resources: list})
}

func (s *Server) handleResourcesRead(req Request) error {
	var params ResourceReadParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return s.writeError(req.ID, newError(ErrInvalidParams, "invalid resource read params", err.Error()))
		}
	}

	if s.getResource == nil {
		return s.writeError(req.ID, newError(ErrInternalError, "no resource provider", nil))
	}

	result, err := s.getResource(s, params.URI)
	if err != nil {
		return s.writeError(req.ID, newError(ErrInternalError, err.Error(), nil))
	}
	return s.writeResult(req.ID, result)
}

func (s *Server) writeResult(id json.RawMessage, result any) error {
	data, err := marshalResult(id, result)
	if err != nil {
		return err
	}
	return s.writeLine(data)
}

func (s *Server) writeError(id json.RawMessage, errObj *ErrorObject) error {
	data, err := marshalError(id, errObj)
	if err != nil {
		return err
	}
	return s.writeLine(data)
}

func (s *Server) writeLine(data []byte) error {
	_, err := s.writer.Write(append(data, '\n'))
	return err
}

// CurrentStore returns the store associated with the server.
func (s *Server) CurrentStore() *core.Store {
	return s.store
}

// ActiveScopePath returns the active EigenMemory scope path for the server.
func ActiveScopePath() (*config.Paths, error) {
	_, paths, err := config.ScopeFromCWD()
	return paths, err
}
