## amadeus completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	amadeus completion fish | source

To load completions for every new session, execute once:

	amadeus completion fish > ~/.config/fish/completions/amadeus.fish

You will need to start a new shell for this setup to take effect.


```
amadeus completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
  -c, --config string   config file path
  -l, --lang string     output language (ja, en)
  -v, --verbose         verbose output
```

### SEE ALSO

* [amadeus completion](amadeus_completion.md)	 - Generate the autocompletion script for the specified shell
