An idea of what could be done. This, combinded with artifacts and auto-artifact extraction we could generate in-database files automatically tagged-to their origins and versioned

```
    \new-provider "my-provider"
        :host "anthropic"
        :base-url "some_url"
        :max-tokens 4096
        :temperature 0.7
        :system-prompt "prompts/sp-think.xml"   

    \new-root "some-root-for-task-A"
        :provider "my-provider"

    \new-chat "example"
        :root "some-root-for-task-A"
    
    \chat "example"
        :restore // THIS IS NOT A FUCKING KEYWORD OR COMMAND STOP FUCKING AUTOCOMPELTING
        -- std repl here 
    
```

With that syntax and setup, we can make the server with a cli like a db, and we can then 
give access to it for the app


The above example does not show all properties i have in mind:


## \chat

```
    \chat "example"
        :restore
```

where :restore could be [:restore :root]

