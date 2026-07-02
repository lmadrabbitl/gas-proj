import { Injectable, signal } from '@angular/core';

export type ToastTone = 'success' | 'error' | 'info';

export type ToastMessage = {
  id: number;
  tone: ToastTone;
  text: string;
};

@Injectable({ providedIn: 'root' })
export class ToastService {
  readonly current = signal<ToastMessage | null>(null);
  private nextID = 1;
  private timeoutID: ReturnType<typeof setTimeout> | null = null;

  success(text: string, duration = 3200): void {
    this.show('success', text, duration);
  }

  error(text: string, duration = 5200): void {
    this.show('error', text, duration);
  }

  info(text: string, duration = 4000): void {
    this.show('info', text, duration);
  }

  clear(): void {
    if (this.timeoutID !== null) {
      clearTimeout(this.timeoutID);
      this.timeoutID = null;
    }
    this.current.set(null);
  }

  private show(tone: ToastTone, text: string, duration: number): void {
    const message = text.trim();
    if (!message) {
      return;
    }

    const id = this.nextID++;
    this.current.set({ id, tone, text: message });

    if (this.timeoutID !== null) {
      clearTimeout(this.timeoutID);
    }

    this.timeoutID = setTimeout(() => {
      if (this.current()?.id === id) {
        this.current.set(null);
      }
      this.timeoutID = null;
    }, duration);
  }
}
