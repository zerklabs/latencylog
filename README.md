lync message aggregation
========================

Messages submitted over the wire will arrive in the following format:

```
user1:<uri>,user2:<uri>| message:from:<uri>,body:<base64 message> [...],at:<time>|header:start:<time>,end:<time>
```

