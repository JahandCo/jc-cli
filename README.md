# Jah and Co- Developer CLI

The developer CLI for the [Jah and Co Developer Platform](https://jahandco.dev) 

## Install

### Linux (x86_64)
```bash
curl -fsSL https://api-s3.jahandco.net/cli-releases/cli/0.1.2/jc_Linux_x86_64.tar.gz | sudo tar -xz -C /usr/local/bin
```
*(ARM64: `jc_Linux_arm64.tar.gz`)*

### macOS
```bash
curl -fsSL https://api-s3.jahandco.net/cli-releases/cli/0.1.2/jc_Darwin_arm64.tar.gz | sudo tar -xz -C /usr/local/bin
```
*(Intel: `jc_Darwin_x86_64.tar.gz`)*

### Windows (PowerShell)
```powershell
Invoke-WebRequest -Uri "https://api-s3.jahandco.net/cli-releases/cli/0.1.2/jc_Windows_x86_64.zip" -OutFile "$env:TEMP\jc.zip"
Expand-Archive -Path "$env:TEMP\jc.zip" -DestinationPath "$env:LOCALAPPDATA\jc" -Force
[Environment]::SetEnvironmentVariable("Path", "$env:Path;$env:LOCALAPPDATA\jc", "User")
```

### From source
```bash
go install .
```

See [`howto.md`](./howto.md) for the full release process if you're
cutting a new version rather than just installing one.

## Usage

```bash
jc login                   # authenticate with the Developer Platform
jc project create my-project  # create a new project
jc init title my-game      # scaffold a new title against the Game SDK
jc dev                     # local development sandbox, viewable from the project console's Dev tab
jc deploy                  # build and deploy the current title's compiled gameplay bundle
jc assets upload <path>    # upload a file into this title's asset library
jc console                 # open the developer web console
```

Run `jc <command> --help` for full flag/argument details on any command.

## Contributing

See [`CLAUDE.md`](./CLAUDE.md) for repo layout, build commands, and CI.

## License

MIT
