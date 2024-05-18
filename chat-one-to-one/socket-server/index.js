const express = require('express');
const http = require('http');
const socketIo = require('socket.io');
const redis = require('redis');
const path = require('path');

// Create an Express application
const app = express();

// Serve static files from the 'public' directory
app.use(express.static(path.join(__dirname, 'public')));

// Create an HTTP server
const server = http.createServer(app);

// Create a Socket.io server
const io = socketIo(server, {
  cors: {
    origin: '*',
  },
});

// Create Redis clients
const REDIS_HOST = process.env.REDIS_HOST || 'localhost';
const REDIS_PORT = process.env.REDIS_PORT || 6379;
const pubClient = redis.createClient({url: `redis://${REDIS_HOST}:${REDIS_PORT}`});
const subClient = redis.createClient({url: `redis://${REDIS_HOST}:${REDIS_PORT}`});

// Handle Redis client errors
pubClient.on('error', (err) => {
  console.error('Redis pubClient error:', err);
});

subClient.on('error', (err) => {
  console.error('Redis subClient error:', err);
});

// Connect Redis clients
(async () => {
  try {
    await pubClient.connect();
    await subClient.connect();

    console.log('pub/sub clients connected to Redis')

    // Subscribe to the 'messages' and 'acknowledgements' channels after connection
    await subClient.subscribe('messages', (message, channel) => {
      const parsedMessage = JSON.parse(message);
      if (channel === 'messages') {
        // Deliver message to the appropriate client
        io.to(parsedMessage.receiverId).emit('receiveMessage', parsedMessage);
      } else if (channel === 'acknowledgements') {
        // Deliver acknowledgment to the appropriate client (sender)
        io.to(parsedMessage.senderId).emit('messageAcknowledged', parsedMessage.messageId);
      }
    });
    
    // Subscribe to the 'acknowledgements' channel
    await subClient.subscribe('acknowledgements', (message, channel) => {
        const parsedMessage = JSON.parse(message);
        io.to(parsedMessage.senderId).emit('messageAcknowledged', parsedMessage.messageId);
      });
  } catch (error) {
    console.error('Error connecting to Redis:', error);
  }
})();

io.on('connection', (socket) => {
  console.log('Client connected:', socket.id);

  // Handle sendMessage event
  socket.on('sendMessage', async (message, callback) => {
    try {
      // Add the sender ID to the message
      message.senderId = socket.id;
      // Publish message to the message broker
      await pubClient.publish('messages', JSON.stringify(message));
      callback({ status: 'sent' });
    } catch (err) {
      console.error('Error publishing message:', err);
      callback({ status: 'error', error: err.message });
    }
  });

  // Handle acknowledgeMessage event
  socket.on('acknowledgeMessage', async (messageId, callback) => {
    try {
      // Handle message acknowledgment (this is a custom event)
      await pubClient.publish('acknowledgements', JSON.stringify({ messageId, senderId: socket.id }));
      callback({ status: 'acknowledged' });
    } catch (err) {
      console.error('Error publishing acknowledgment:', err);
      callback({ status: 'error', error: err.message });
    }
  });

  socket.on('disconnect', () => {
    console.log('Client disconnected:', socket.id);
  });
});

// Start the server on port 3000
const PORT = process.env.PORT || 3000;
server.listen(PORT, () => {
  console.log(`Server is running on port ${PORT}`);
});
