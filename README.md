# Assistant Kernel (akernel)

A minimal, resource-efficient HTTP daemon that adds state management and tools to Ollama and OpenRouter. 
Fully written in Go. It also provides an easy-to-use abstraction for using AI models.

## Key Features
- Persistent chat history with SQLite
- Ability to create and provide tools to the model
- System prompt loaded from `$XDG_CONFIG_HOME/akernel/system.md`
- Optimized to be a local daemon with minimal resource usage
- Fully documented OpenAPI spec for easy usage.
- Works with both Ollama and OpenRouter

## Quick Start
For this example, ensure [Ollama](https://ollama.com/) is installed and running.

### Create an example config
```bash
mkdir $HOME/.config/akernel
echo "models = [ { name = "gemma4:e4b", id = "gemma4:e4b-it-qat", description = "a small model", provider = "ollama" } ]" >> $HOME/.config/akernel/config.toml
```

### Clone the repository and run akernel
```bash
git clone https://codeberg.org/efournierrobert/akernel.git
go build -o akernel cmd/main.go
./akernel
```

> **Note:** akernel binds to `0.0.0.0:50500`, so it is reachable from other machines on your local network. There is no authentication — this is intentional for local/daemon use.

## Example usage
```bash
# Start a new conversation using conversationId: 0
curl -X POST http://localhost:50500/chat -d '{"model": "gemma4:e4b", "message": "Could you tell me what is 2 + 2?", "conversationId": 0}'

# Response: [
# {
#   "role":"assistant",
#   "conversationId":6,
#   "message":"2 + 2 = 4",
#   "thinking":"The user is asking a simple arithmetic question:...",
#   "conversationTokenCount":178
#  }
# ]  
```

## Documentation
- Full API specification is available at `openapi/doc.yaml`
- Reference CLI implementation can be found in `example/acli.go`
- Configuration file documentation can be found in `docs/` 