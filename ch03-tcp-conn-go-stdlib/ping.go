package ch03

// ## Implementing a Heartbeat
// For long-running network connections that may experience extended idle periods at the application level,
// it’s wise to implement a heartbeat between nodes to advance the deadline.
//	- This allows you to quickly identify network issues and promptly reestablish a connection as opposed
// 	  to waiting to detect the network error when your application goes to transmit data.
//	- In this way, you can help make sure your application always has a good network connection when it needs it.
// For our purposes, a heartbeat is a message sent to the remote side with the intention of eliciting a reply,
// which we can use to advance the deadline of our network connection.
// 	- Nodes send these messages at a regular interval, like a heartbeat.
//	- Not only is this method portable over various operating systems,
//    but it also makes sure the application using the network connection is responding, since the application implements the heartbeat.
//  - Also, this technique tends to play well with firewalls that may block TCP keepalives. We’ll discuss keepalives in Chapter 4.
// To start, you’ll need a bit of code you can run in a goroutine to ping at regular intervals.
//	- You don’t want to needlessly ping the remote node when you recently received data from it,
//	- so you need a way to reset the ping timer.
//	- Listing 3-10 is a simple implementation from a file named ping.go that meets those requirements.
// I use ping and pong messages in my heartbeat examples, where the reception of a ping message—the challenge—tells
//	- the receiver it should reply with a pong message—the response.
//	- The challenge and response messages are arbitrary.
//	- You could use anything you want to here, provided the remote node knows your intention is to elicit its reply.
