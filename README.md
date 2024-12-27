Pubsub server, basically just a hashmap with mutex over tcp

TCP PubSub Protocol
------------------
```
PUBLISH (0x00):
+---------+--------------+----------+--------------+---------+
| cmd (1) | chan_len (1) | channel  | msg_len (4) | payload |
+---------+--------------+----------+--------------+---------+
|   0x00  |     0xNN     | N bytes  |    0xNNNN   | N bytes |
+---------+--------------+----------+--------------+---------+

SUBSCRIBE (0x01): 
+---------+--------------+----------+
| cmd (1) | chan_len (1) | channel  |
+---------+--------------+----------+
|   0x01  |     0xNN     | N bytes  |
+---------+--------------+----------+
```

Notes:
- All integers are little-endian
- Channel length is 1 byte (max 255 bytes)
- Message length is 4 bytes (max 4GB, kind of crazy, but it's fine)
- Connection stays open for subscribers
- Publishers can send multiple publish commands before disconnecting
