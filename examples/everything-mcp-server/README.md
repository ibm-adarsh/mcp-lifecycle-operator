# Everything MCP Server Example

This example deploys the Everything MCP Server, one of the [reference servers](https://github.com/modelcontextprotocol/servers) from the Model Context Protocol project. It exercises all MCP features including prompts, resources, and tools.

Source code: https://github.com/modelcontextprotocol/servers/tree/main/src/everything

## Deployment

```bash
kubectl apply -f mcpserver.yaml
```

This creates:
- Deployment running the MCP server
- Service exposing port 3001

Check status:

```bash
kubectl get mcpserver everything-mcp-server
```

## Testing

Port-forward to the service:

```bash
kubectl port-forward svc/everything-mcp-server 3001:3001
```

Connect with an MCP client at `http://localhost:3001/mcp`.

## Cleanup

```bash
kubectl delete -f mcpserver.yaml
```
