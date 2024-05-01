# Direct endpoint

Direct endpoint implements Endpoint interface by extending BaseEndpoint type.
This endpoint allows communication between code running in the same process by utilizing channels internally.

It uses a dndm.Linker to link Intents and Interests together internally.

Intent and Interest Routes must match for the link to happen, meaning that both the path and the message types
must match exactly.
