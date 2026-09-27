/** Each binary frame carries one Noise message. */
export interface FrameSocket {
  send(frame: Uint8Array<ArrayBuffer>): void;
  /** Rejects with SocketClosed only once every frame received was read, or with SocketTimeout. */
  receive(): Promise<Uint8Array<ArrayBuffer>>;
  close(): void;
}

/** Opens a connection to a URL, or rejects with SocketUnavailable. */
export type SocketFactory = (url: string) => Promise<FrameSocket>;

/** The connection never opened: nothing listens on the port, or the server refused the upgrade. */
export class SocketUnavailable extends Error {
  constructor() {
    super("The connection to Ravenpass could not be opened.");
    this.name = "SocketUnavailable";
  }
}

export class SocketClosed extends Error {
  readonly code: number;

  constructor(code: number) {
    super(`Ravenpass closed the connection with code ${code}.`);
    this.name = "SocketClosed";
    this.code = code;
  }
}

export class SocketTimeout extends Error {
  constructor() {
    super("Ravenpass did not answer in time.");
    this.name = "SocketTimeout";
  }
}

const openTimeoutMs = 5_000;
// Above the desktop app's handshake limit and its share progress interval, both ten seconds.
const frameTimeoutMs = 15_000;
const closeFinished = 1000;
const closeMalformed = 4400;

interface Waiter {
  resolve: (frame: Uint8Array<ArrayBuffer>) => void;
  reject: (error: Error) => void;
}

/** A browser WebSocket that keeps the frames it receives until they are asked for. */
export class LinkSocket implements FrameSocket {
  private readonly socket: WebSocket;
  private readonly frames: Uint8Array<ArrayBuffer>[] = [];
  private waiter: Waiter | null = null;
  private closed: SocketClosed | null = null;

  private constructor(socket: WebSocket) {
    this.socket = socket;
    socket.binaryType = "arraybuffer";
    socket.addEventListener("message", (event: MessageEvent<unknown>) => {
      if (!(event.data instanceof ArrayBuffer)) {
        socket.close(closeMalformed);
        return;
      }
      const frame = new Uint8Array(event.data);
      if (this.waiter) {
        this.waiter.resolve(frame);
        this.waiter = null;
      } else {
        this.frames.push(frame);
      }
    });
    socket.addEventListener("close", (event) => {
      this.closed = new SocketClosed(event.code);
      this.waiter?.reject(this.closed);
      this.waiter = null;
    });
  }

  static connect(url: string): Promise<LinkSocket> {
    return new Promise((resolve, reject) => {
      const socket = new WebSocket(url);
      const connection = new LinkSocket(socket);
      const timer = setTimeout(() => {
        socket.close();
        reject(new SocketUnavailable());
      }, openTimeoutMs);
      socket.addEventListener("open", () => {
        clearTimeout(timer);
        resolve(connection);
      });
      socket.addEventListener("close", () => {
        clearTimeout(timer);
        reject(new SocketUnavailable());
      });
    });
  }

  send(frame: Uint8Array<ArrayBuffer>): void {
    this.socket.send(frame);
  }

  receive(): Promise<Uint8Array<ArrayBuffer>> {
    const frame = this.frames.shift();
    if (frame) return Promise.resolve(frame);
    if (this.closed) return Promise.reject(this.closed);
    return new Promise((resolve, reject) => {
      const timer = setTimeout(() => {
        this.waiter = null;
        this.close();
        reject(new SocketTimeout());
      }, frameTimeoutMs);
      this.waiter = {
        resolve: (next) => {
          clearTimeout(timer);
          resolve(next);
        },
        reject: (error) => {
          clearTimeout(timer);
          reject(error);
        },
      };
    });
  }

  close(): void {
    this.socket.close(closeFinished);
  }
}
