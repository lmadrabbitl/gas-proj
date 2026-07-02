import { HttpClient } from '@angular/common/http';
import { Injectable, computed, signal } from '@angular/core';
import { tap } from 'rxjs';

import { LoginResponse, RegisterPayload, RegisterResponse } from '../shared/models';

const TOKEN_KEY = 'gas-proj.auth.token';

@Injectable({ providedIn: 'root' })
export class AuthService {
  private readonly tokenSignal = signal<string | null>(this.readToken());
  readonly isLoggedIn = computed(() => Boolean(this.tokenSignal()));

  constructor(private readonly http: HttpClient) {}

  login(email: string, password: string) {
    return this.http.post<LoginResponse>('/api/auth/login', { email, password }).pipe(
      tap((response) => this.setToken(response.token)),
    );
  }

  register(payload: RegisterPayload) {
    return this.http.post<RegisterResponse>('/api/auth/register', payload);
  }

  token(): string | null {
    return this.tokenSignal();
  }

  logout(): void {
    localStorage.removeItem(TOKEN_KEY);
    this.tokenSignal.set(null);
  }

  private setToken(token: string): void {
    localStorage.setItem(TOKEN_KEY, token);
    this.tokenSignal.set(token);
  }

  private readToken(): string | null {
    return localStorage.getItem(TOKEN_KEY);
  }
}
