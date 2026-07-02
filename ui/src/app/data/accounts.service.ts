import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { map } from 'rxjs';

import { Account, CreateAccountPayload, UpdateAccountPayload } from '../shared/models';

@Injectable({ providedIn: 'root' })
export class AccountsService {
  constructor(private readonly http: HttpClient) {}

  list() {
    return this.http.get<{ accounts: Account[] }>('/api/accounts').pipe(map((res) => res.accounts));
  }

  create(payload: CreateAccountPayload) {
    return this.http.post<{ account: Account }>('/api/accounts', payload).pipe(map((res) => res.account));
  }

  update(code: string, payload: UpdateAccountPayload) {
    return this.http.patch<{ account: Account }>(`/api/accounts/${code}`, payload).pipe(map((res) => res.account));
  }

  reorder(codes: string[]) {
    return this.http.patch<void>('/api/accounts/reorder', { codes });
  }

  deactivate(code: string) {
    return this.http.delete<void>(`/api/accounts/${code}`);
  }

  deletePermanent(code: string) {
    return this.http.delete<void>(`/api/accounts/${code}/permanent`);
  }
}
