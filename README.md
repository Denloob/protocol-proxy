# Protocol Proxy
Manually controlled proxy for exploring/debugging TCP protocols

**Early WIP**

I am planning to make it a TUI-controlled interactive TCP proxy, which would
allow the user to explore, filter, replace, log, search, etc. etc. packages sent
from/to the client, and view them both as a hexdump, as a string, and as a
combination of both

Currently there's a simple TUI which supports some of the options, as can be seen
in the [Demo](#demo) at the bottom of the README

## Building and running
Clone the project, `go build`, and run the `protocol-proxy` executable

## Contributing
Any help is appreciated!

Contribute by creating an Issue/a Pull Request

## Demo
```
$ ./protocol-proxy -in-port 1337 -out-port 8080
in->out: received 20 bytes. View? (Y/n) y
Action? [D]rop/view [H]ex/view hexdum[P]/view [S]trings/he[X] overwrite/[A]scii overwrite/open in [E]ditor/[N]othing
p
00000000  4d 65 73 73 61 67 65 20  66 72 6f 6d 20 63 6c 69  |Message from cli|
00000010  65 6e 74 0a                                       |ent.|

You may interact with the same packet again.
View? (Y/n) y
Action? [D]rop/view [H]ex/view hexdum[P]/view [S]trings/he[X] overwrite/[A]scii overwrite/open in [E]ditor/[N]othing
a
NOTE: use \n for new lines, \\n for literal '\n'. Entering a new line will send the packet.
Message from proxy
You may interact with the same packet again.
View? (Y/n) n
out->in: received 20 bytes. View? (Y/n) y
Action? [D]rop/view [H]ex/view hexdum[P]/view [S]trings/he[X] overwrite/[A]scii overwrite/open in [E]ditor/[N]othing
s
1 strings found
---
Message from server

You may interact with the same packet again.
View? (Y/n) y
Action? [D]rop/view [H]ex/view hexdum[P]/view [S]trings/he[X] overwrite/[A]scii overwrite/open in [E]ditor/[N]othing
d
The packet was dropped.
```

In the Demo the server ended up receiving `Message from proxy` and the client
didn't get any answer at all.

All the currently listed options are supported. (My favorite is `open in editor`,
which allows you to modify the text/binary data in any editor, including hex editors).
