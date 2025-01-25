# brunch

Requires:

- `ANTHROPIC_API_KEY` environment variable set

```bash
 ~/workspace/brunch/ [mods*] make
go build -o bru ./cmd/bru

 ~/workspace/brunch/ [mods*] ./bru
new chat


                W E L C O M E

        To see a list of commands type '\?'


Chat started. Press Ctrl+C to exit and view conversation tree.
Enter your messages (press Enter twice to send):

55eb949f> \?
handleCommand: \?
Commands:
        \l: List chat history [current branch of chat]
        \t: List chat tree [all branches]
        \i: Queue image [import image file into chat for inquiry]
        \s: Save snapshot [save a snapshot of the current tree to disk]
        \p: Go to parent [traverse up the tree]
        \c: Go to child [traverse down the tree to the nth child]
        \r: Go to root [traverse to the root of the tree]
        \g: Go to node [traverse to a specific node by hash]
        \.: List children [list all children of the current node]
        \x: Toggle chat [toggle chat mode on/off - chat on by default press enter twice to send with no command leading]
        \q: Quit [save and quit]

55eb949f>
```


```
1. `\new-provider "name"`
   - Creates a new provider
   - Required properties:
     - `:host` (string)
     - `:base-url` (string)
     - `:max-tokens` (integer)
     - `:temperature` (real/float)
     - `:system-prompt` (string)

2. `\new-chat "name"`
   - Creates a new chat
   - Required properties:
     - `:provider` (string)

3. `\chat "name"`
   - Interacts with an existing chat
   - Optional properties:
     - `:hash` (string)
```
