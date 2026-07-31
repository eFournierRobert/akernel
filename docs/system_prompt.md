# System Prompt

A system prompt is a special prompt that is automatically inserted when making a request to an LLM model. It provides general
guidance to it and the vast majority of models are trained to follow those instructions.

## Path

In Assistant Kernel, all models share the same system prompt. It needs to be defined in `$XDG_CONFIG_HOME/akernel/system.md`.

Note that it will be sent to the model when making a request so it will consume tokens from the context.

**`$XDG_CONFIG_HOME` is usually just `$HOME/.config/`.*