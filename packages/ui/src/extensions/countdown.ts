/** Counts whole seconds down to a moment in Unix milliseconds. */
export class Countdown {
  private readonly until: number;

  constructor(until: number) {
    this.until = until;
  }

  /** Rounded up and never below zero, so the last second counts until it has passed. */
  secondsLeft(now: number): number {
    return Math.max(0, Math.ceil((this.until - now) / 1000));
  }

  expired(now: number): boolean {
    return this.secondsLeft(now) === 0;
  }

  /** Minutes and two-digit seconds, such as `4:59`. */
  display(now: number): string {
    const left = this.secondsLeft(now);
    const minutes = Math.floor(left / 60);
    const seconds = String(left % 60).padStart(2, "0");
    return `${minutes}:${seconds}`;
  }
}
