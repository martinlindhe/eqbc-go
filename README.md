# EQ Box Chat. Private Chat Relay server for EQ bots

Re-implementation of eqbcs2 in golang for mq2 / PEQ-TGC

Tested with MQ2EQBC 15.0503, as distributed with the PEQ MQ2.





## Improvements vs. stock eqbcs2.exe (peq mq2 build)

- No upper limit to connected clients (eqbcs2.exe has a limit of 55 clients)

- --timestamp flag decorates log with timestamps

- Colorized output

- [Docker ready](https://hub.docker.com/repository/docker/martinlindhe/eqbc-go)
