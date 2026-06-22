# Assistant Kernel

Assistant kernel (akernel) is a minimal HTTP service, meant to run as a daemon, that lets you
interact with Ollama. It manages conversations and has tools available to the AI model of your choice.
It is front end agnostic so you can use any front end that implements the API.

Since it is supposed to run as a daemon locally, it was made to take as little resources as possible.
The conversation database is an SQLite database stored in `$XDG_DATA_HOME/akernel/akernel.db`.

## Features
Main features of akernel:
- Interaction with Ollama locally for AI inference
- Conversation storage
- Context summarization activated manually
- Tools support

## Documentation
There's a full OpenAPI documentation in `openapi/doc.yaml` if you want to make an interface for it
and there's a CLI example in `example/acli/` written in Rust.