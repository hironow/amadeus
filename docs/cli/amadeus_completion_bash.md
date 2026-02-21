## amadeus completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(amadeus completion bash)

To load completions for every new session, execute once:

#### Linux:

	amadeus completion bash > /etc/bash_completion.d/amadeus

#### macOS:

	amadeus completion bash > $(brew --prefix)/etc/bash_completion.d/amadeus

You will need to start a new shell for this setup to take effect.


```
amadeus completion bash
```

### Options

```
  -h, --help              help for bash
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
