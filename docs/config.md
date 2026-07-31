# Configuration

Assistant Kernel needs a small amount of TOML configuration to work fully. 
That configuration if defined in `$XDG_CONFIG_HOME/akernel/config.toml` and won't be automatically generated at first launch.

That configuration has two parts: models and keys. We'll go through each one.

## Models

`models` is an array of objects containing the models you want to expose. Akernel doesn't expose all the models you can use via
Ollama or OpenRouter to keep the list short and simple. Making what's being exposed deliberate. 

Here are the fields of the model object. Each element in `models` needs to contain those fields.

| Name   | Description                                                                                                   |
|--------|---------------------------------------------------------------------------------------------------------------|
| name | The name of the model that will be exposed.                                                                   |
| id | The real name of the model for the provider. Like `gemma4:26b-a4b-it-qat ` or `z-ai/glm-5.2`.                 |
| description | A small description that will be sent with the name of the model when requesting the model list. | 
| provider | The provider of the model. Basically who to send the actual requests to. Can be `"ollama"` or `"openrouter"`. |

## Keys

This part contains the API keys you need to provide to Assistant Kernel if you want to use those features. Currently,
there's two you can give:
- `brave_api_key`: The API key for the web search tool. If it isn't provided, `web_search` won't be available.
- `open_router_key`: The API key for OpenRouter. If you configure models from OpenRouter, you need to also provide this key.

## Example
```toml
models = [
  { name = "science", id = "qwen3.6:35b", description = "Model for sciences", provider = "ollama" },
  { name = "big science", id = "z-ai/glm-5.2", description = "Model for bigger sciences", provider = "openrouter" }, 
  { name = "chatter", id = "gemma4:26b-a4b-it-qat", description = "Model for yapping", provider = "ollama"},
  { name = "big chatter", id = "openai/gpt-5.6-luna", description = "Model for bigger yapping", provider = "openrouter" },
]

brave_api_key = "[BRAVE API KEY]"
open_router_key = "[OPENROUTER API KEY]"
```
