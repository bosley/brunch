# brunch

## Install
Requires:

- `ANTHROPIC_API_KEY` environment variable set


```bash
make
./brucli
```

## Example Usage

Then, we can start submitting statements to do things like "make a new chat session," and "derive alternative provider configurations."

Here is an overview of the initial statement options that help define chats and providers:

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
     - `:hash` (string) [loads a specified hash, defaults to latest]

4. `\new-ctx "name"`
   - Creates a new context for knowledge/data access
   - Optional properties (at least one required):
     - `:dir` (string) [directory path for file access]
     - `:database` (string) [database connection string]
     - `:web` (string) [web endpoint]
```

Example of the creating a chat, and using the chat REPL:

```bash
 ~/workspace/brunch/ [main] ./brucli
time=2025-01-26T12:26:35.102-05:00 level=INFO msg="installing core" dir=/tmp/brunch
>\new-chat "mychat" :provider "anthropic"
>\chat "mychat"


                        W E L C O M E

                Send a message to the assistant by
                typing the message and pressing "enter"
                twice.

                To see a list of commands type '\?'
                To quit, type '\q'

Chat started. Press Ctrl+C to exit and view conversation tree.
Enter your messages (press Enter twice to send):
```

Now the chat is loaded, so lets see what the chat REPL commands are:

```bash
[9809d4c7]>  \?
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
        \a: List artifacts [display artifacts from current node] or [write artifacts to disk if followed by a directory path]
        \q: Quit [save and quit]
        \new-k: Attach new knowledge-context [attach a non-existing knowledge-context to the chat]
        \attach-k: Attach existing knowledge-context [attach an existing knowledge-context to the chat]
```

Image analysis:

```bash
[9809d4c7]>  \i
Enter image path:
assets/test_image.jpg

[9809d4c7]>  can you describe this image please ?

assistant>  This image appears to be a stylized, geometric portrait illustration of a man with distinct facial features. The man is portrayed with angular lines and shapes that create a cubist or faceted effect, reminiscent of pop art styles.

The illustration uses bold, contrasting colors like orange, yellow, green, and shades of blue. The man's face is composed of geometric shapes and planes that give it a three-dimensional, almost sculptural quality.

His eyes are highlighted with bright blue shapes, making them stand out prominently. The overall style suggests a modern, graphic design approach, potentially used for a magazine cover, poster, or some kind of promotional material.

While I cannot identify the specific individual depicted, the striking visual style and use of vibrant colors create an eye-catching and memorable portrait illustration.

[06113e43]>
```

Now, lets add another message and see the "tree":

```bash
[06113e43]>  Thanks, that was spot-on. Tell me some cool facts about programming, but keep it brief

assistant>  Sure, here are some cool brief facts about programming:

1. The first computer programmer was a woman named Ada Lovelace, who wrote the first algorithm for a computing machine in the 1840s.

2. The first computer bug was a real bug - a moth stuck in the Harvard Mark II computer in 1947, which led to the term "debugging."

3. The programming language Python was named after the British comedy group Monty Python, not the snake.

4. The "Hello, World!" program is a traditional way to introduce new programmers to a programming language and is often the first program written.

5. The most popular programming language in the world is JavaScript, which is used to add interactivity and dynamic effects to websites.

6. The first computer game, "Spacewar!," was created in 1962 by a group of programmers at MIT.

7. The term "hacker" originally referred to someone who was a skilled programmer and enjoyed exploring the details of programming systems.

[27377479]>  \t
[ROOT] Provider: mychat, Model: claude-3-sonnet-20240229
├── Temperature: 0.70
├── MaxTokens: 4000
└── Hash: 9809d4c779c7a9aef9cfa2f287219d42829beee5b62c26ad5ef8e3def83e1f88
    └── [MESSAGE_PAIR] Time: 2025-01-26 12:31:35
        ├── User (user): can you describe this ima...
        ├── User Images: assets/test_image.jpg
        ├── Assistant (assistant): This image appears to be ...
        └── Hash: 06113e434abff4380d3c4761033b2b2ecb09b08558fc9a1dfaa2f33c224a7db1
        └── [MESSAGE_PAIR] Time: 2025-01-26 12:32:41
            ├── User (user): Thanks, that was spot-on....
            ├── Assistant (assistant): Sure, here are some cool ...
            └── Hash: 273774790c3a95e035305ed707f992e44a6253412411ef778aeb7b41f29db0b6


[27377479]>
```

Hmm, that was a dumb question and I don't want to start all over! The context is now polluted GAHH DARN.. oh waitt

```bash
[06113e43]>  \g 9809d4c779c7a9aef9cfa2f287219d42829beee5b62c26ad5ef8e3def83e1f88

[9809d4c7]>  \t
[ROOT] Provider: mychat, Model: claude-3-sonnet-20240229
├── Temperature: 0.70
├── MaxTokens: 4000
└── Hash: 9809d4c779c7a9aef9cfa2f287219d42829beee5b62c26ad5ef8e3def83e1f88
    └── [MESSAGE_PAIR] Time: 2025-01-26 12:31:35
        ├── User (user): can you describe this ima...
        ├── User Images: assets/test_image.jpg
        ├── Assistant (assistant): This image appears to be ...
        └── Hash: 06113e434abff4380d3c4761033b2b2ecb09b08558fc9a1dfaa2f33c224a7db1
        └── [MESSAGE_PAIR] Time: 2025-01-26 12:32:41
            ├── User (user): Thanks, that was spot-on....
            ├── Assistant (assistant): Sure, here are some cool ...
            └── Hash: 273774790c3a95e035305ed707f992e44a6253412411ef778aeb7b41f29db0b6


[9809d4c7]>  \l

[9809d4c7]>  Howdy partner! lets try some more!

assistant>  Howdy! Im ready to continue our conversation. Feel free to ask me anything or share your thoughts.

[dbe4cb25]>  \t
[ROOT] Provider: mychat, Model: claude-3-sonnet-20240229
├── Temperature: 0.70
├── MaxTokens: 4000
└── Hash: 9809d4c779c7a9aef9cfa2f287219d42829beee5b62c26ad5ef8e3def83e1f88
    │├── [MESSAGE_PAIR] Time: 2025-01-26 12:31:35
    │    ├── User (user): can you describe this ima...
    │    ├── User Images: assets/test_image.jpg
    │    ├── Assistant (assistant): This image appears to be ...
    │    └── Hash: 06113e434abff4380d3c4761033b2b2ecb09b08558fc9a1dfaa2f33c224a7db1
    │    └── [MESSAGE_PAIR] Time: 2025-01-26 12:32:41
    │        ├── User (user): Thanks, that was spot-on....
    │        ├── Assistant (assistant): Sure, here are some cool ...
    │        └── Hash: 273774790c3a95e035305ed707f992e44a6253412411ef778aeb7b41f29db0b6
    └── [MESSAGE_PAIR] Time: 2025-01-26 12:34:37
        ├── User (user): Howdy partner! lets try s...
        ├── Assistant (assistant): Howdy! Im ready to conti...
        └── Hash: dbe4cb25ca1bd5b9a30e8cb1e0ae1af9ed9745c604246f44d796d8ff429a1cd2
```

Nice, now we have both and we can hop betweeen these two contrived examples!

## Producing Artifacts

Here is an example of retrieving artifacts from a node:

```bash
[9809d4c7]>  please generate me some Haskell code!

... omitted for bevity ...


[988f36fc]>  \a
Artifacts in current node:
        0: Text: Sure, here's some Haskell code that defines a simp...
        1: File [haskell] Name: <unnamed artifact>
           Preview: -- Define a function to find the sum of a list of ...
        2: Text: Here's how the code works:

1. The function `sumLi...

[988f36fc]>

```

Now lets see what we can get:

```bash
[988f36fc]>  \a /tmp/go-is-better

[988f36fc]>  \q
saving back to loaded snapshot
>\q
 ~/workspace/brunch/ [main*] tree /tmp/go-is-better
/tmp/go-is-better
├── 32b260ec.artifact
├── 91e68199.artifact
└── file_113.artifact.haskell

1 directory, 3 files
 ~/workspace/brunch/ [main*]

 ~/workspace/brunch/ [main*] cat /tmp/go-is-better/file_113.artifact.haskell
-- Define a function to find the sum of a list of integers
sumList :: [Int] -> Int
sumList [] = 0
sumList (x:xs) = x + sumList xs

-- Example usage
main = do
  print (sumList [1, 2, 3, 4, 5]) -- Output: 15
  print (sumList [])              -- Output: 0
```
