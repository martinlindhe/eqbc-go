# EQBC protocol notes


port : 2112  is default port




## Packet format

The first login command:   "LOGIN=name;" or "LOGIN:password=name;"


All commands after LOGIN is in the following format:

    \tCMD\n
    payload\n

// eqbcs msg types
#define CMD_DISCONNECT "\tDISCONNECT\n"
#define CMD_NAMES "\tNAMES\n"
#define CMD_PONG "\tPONG\n"
#define CMD_MSGALL "\tMSGALL\n"
#define CMD_TELL "\tTELL\n"
#define CMD_CHANNELS "\tCHANNELS\n"
#define CMD_LOCALECHO "\tLOCALECHO "
#define CMD_BCI "\tBCI\n"




## /bccmd connect <server> <port> <password>

client command:
    /bccmd connect 127.0.0.1 2112

CLIENT SENDS:
    "LOGIN=User1;"

SERVER BROADCASTS:
    "\tNBJOIN=User1\n"
    "\tNBCLIENTLIST=User1 User2\n"

CLIENT SENDS:
    "\tLOCALECHO 1\n"

SERVER SENDS:
    "-- Local Echo: ON\n"




## DISCONNECT

client command:
    /bccmd quit

CLIENT SENDS:
    "\tDISCONNECT\n" + 0x00

SERVER BROADCASTS:
    "\tNBQUIT=User1\n"
    "\tNBCLIENTLIST=User2 User3\n"




## NBMSG

CLIENT SENDS:
    "\tNBMSG\n"
    "[NB]|data|[NB]\n"

SERVER BROADCASTS:
    "\tNBPKT:User1:[NB]|data|[NB]\n"




## /bct

client command:
    /bct User2 //say hello

CLIENT SENDS:
    "\tTELL\n"
    "User2 //say hello\n"

SERVER SENDS TO "User2":
    "[User1] //say hello\n"




## /bca, /bcaa

NOTE: /bcaa local command is handled in MQ2EQBC.dll.
The server data is otherwise identical to /bca


client command:
    /bca //say hello

CLIENT SENDS:
    "\tMSGALL\n"
    "//say hello\n"

SERVER SENDS TO each other user:
    "<User1> User2 //say hello\n"




## /bcg, /bcga

NOTE: Is translated to /bct commands in MQ2EQBC.dll




## JOIN CHANNELS

set the list of channels to receive tells from

client command:
    /bccmd channels chan1 chan2

CLIENT SENDS:
    "\tCHANNELS\n"
    "chan1 chan2\n"

SERVER RESPONDS:
    "Client joined channels chan1 chan2.\n"




## SEND TELLS TO CHANNELS

send cmd to all in channel "chan1":
    /bct chan1 //say hello

CLIENT SENDS:
    "\tTELL\n"
    "chan1 //say hello\n"




## Client names

client command:
    /bccmd names

CLIENT SENDS:
    "\tNAMES\n"

SERVER RESPONDS:
    "-- Names: User1, User2, User3.\n"




# PING / PONG

server sends:
    "\tPING\n"

client responds:
    "\tPONG\n"
    "\n"
