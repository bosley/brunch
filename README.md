# brunch

A configurable REPL interface for AI model interactions, with conversation state management and navigation capabilities.

## Features

- Configurable AI provider support (currently Anthropic Claude)
- Conversation tree navigation and state management
- Image context support
- Built-in commands for history and tree traversal

## Quick Start

0. Build the example:

```bash
go build ./examples/basic 
```

1. Set your API key:
```bash
export ANTHROPIC_API_KEY='your-key-here'
```

2. Initialize a new configuration:
```bash

./basic -init .
```

3. Start the REPL:
```bash
./basic -load . 
```

or 

```bash
./basic
```

## Commands

- `\h` - Show current conversation hash that the next message will branch from
- `\l` - View conversation history along the current branch
- `\t` - Display entire conversation tree (condensed content)
- `\r` - List navigation routes (parent, child-0, child-1, etc)
- `\i` - Queue images to be sent with the next message from user (any N files can be queued sequentially)

## Configuration

Configuration for example is managed via `brunch.yaml`. See the examples directory for sample configurations, but no configuration
is tied to the library itself. The example is just a simple way to get started.

## License

See LICENSE file for details.
