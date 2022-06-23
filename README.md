# knowledge-base

wip

## Installation

```
$ go build
$ sudo cp knowledge-base /usr/local/bin/
$ cp ./_contrib/knowledge-base.service ~/.config/systemd/user/
$ systemctl --user daemon-reload
$ systemctl --user enable --now knowledge-base.service
$ journalctl --user-unit knowledge-base.service
```

## Development
### Conventional Commits

Install `pre-commit`

```
$ pip install pre-commit
```

Install the hook

```
$ pre-commit install --hook-type commit-msg
```
