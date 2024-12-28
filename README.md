Pubsub server, basically just a hashmap with mutex over tcp

PubSub Protocol
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

UNSUBSCRIBE (0x02):
+---------+--------------+----------+
| cmd (1) | chan_len (1) | channel  |
+---------+--------------+----------+
|   0x02  |     0xNN     | N bytes  |
+---------+--------------+----------+

Message Format (Server -> Subscriber):
+--------------+----------+--------------+---------+
| chan_len (1) | channel  | msg_len (4) | payload |
+--------------+----------+--------------+---------+
|     0xNN     | N bytes  |    0xNNNN   | N bytes |
+--------------+----------+--------------+---------+
```

Notes:
- All integers are little-endian
- Channel length is 1 byte (max 255 bytes) - suitable for topic identifiers
- Message length is 4 bytes (max 4GB)
- Connection stays open for subscribers to receive messages
- Subscribers receive messages in the Message Format (without command byte)
- Publishers can send multiple publish commands before disconnecting
- Subscribers can subscribe/unsubscribe to multiple channels on same connection
- Simple hashmap-based server with mutex for thread safety
- Built on reliable TCP transport - no additional error checking needed
- Zero-copy message forwarding from publishers to subscribers
