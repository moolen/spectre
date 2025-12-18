import { Observable } from 'rxjs';

export interface Rpc {
  request(service: string, method: string, data: Uint8Array): Promise<Uint8Array>;
  clientStreamingRequest(service: string, method: string, data: Observable<Uint8Array>): Promise<Uint8Array>;
  serverStreamingRequest(service: string, method: string, data: Uint8Array): Observable<Uint8Array>;
  bidirectionalStreamingRequest(service: string, method: string, data: Observable<Uint8Array>): Observable<Uint8Array>;
}

export class GrpcWebTransport implements Rpc {
  private baseUrl: string;

  constructor(baseUrl: string) {
    this.baseUrl = baseUrl;
  }

  request(service: string, method: string, data: Uint8Array): Promise<Uint8Array> {
    throw new Error('Unary requests not implemented');
  }

  clientStreamingRequest(service: string, method: string, data: Observable<Uint8Array>): Promise<Uint8Array> {
    throw new Error('Client streaming not implemented');
  }

  serverStreamingRequest(service: string, method: string, data: Uint8Array): Observable<Uint8Array> {
    return new Observable((subscriber) => {
      const url = `${this.baseUrl}/${service}/${method}`;
      
      fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/grpc-web+proto',
          'X-Grpc-Web': '1',
        },
        body: this.encodeGrpcWebRequest(data),
      })
        .then(async (response) => {
          if (!response.ok) {
            throw new Error(`gRPC request failed: ${response.status} ${response.statusText}`);
          }

          const reader = response.body?.getReader();
          if (!reader) {
            throw new Error('Response body is not readable');
          }

          let buffer = new Uint8Array(0);

          while (true) {
            const { done, value } = await reader.read();
            
            if (done) {
              break;
            }

            // Append new data to buffer
            const newBuffer = new Uint8Array(buffer.length + value.length);
            newBuffer.set(buffer);
            newBuffer.set(value, buffer.length);
            buffer = newBuffer;

            // Process complete messages from buffer
            while (buffer.length >= 5) {
              // gRPC-Web frame format: [compression-flag: 1 byte][message-length: 4 bytes][message: N bytes]
              const compressionFlag = buffer[0];
              const messageLength = new DataView(buffer.buffer, buffer.byteOffset + 1, 4).getUint32(0, false);
              
              if (buffer.length < 5 + messageLength) {
                // Not enough data yet for complete message
                break;
              }

              // Extract message
              const message = buffer.slice(5, 5 + messageLength);
              
              // Remove processed message from buffer
              buffer = buffer.slice(5 + messageLength);

              // Skip trailers (compression flag 0x80)
              if (compressionFlag === 0x80) {
                continue;
              }

              subscriber.next(message);
            }
          }

          subscriber.complete();
        })
        .catch((error) => {
          subscriber.error(error);
        });
    });
  }

  bidirectionalStreamingRequest(service: string, method: string, data: Observable<Uint8Array>): Observable<Uint8Array> {
    throw new Error('Bidirectional streaming not implemented');
  }

  private encodeGrpcWebRequest(message: Uint8Array): Uint8Array {
    // gRPC-Web frame format: [compression-flag: 1 byte][message-length: 4 bytes][message: N bytes]
    const frame = new Uint8Array(5 + message.length);
    frame[0] = 0; // No compression
    new DataView(frame.buffer).setUint32(1, message.length, false); // Big-endian
    frame.set(message, 5);
    return frame;
  }
}
