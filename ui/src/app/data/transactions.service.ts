import { HttpClient, HttpParams } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { map } from 'rxjs';

import { BulkTransactionUpdatePayload, Transaction, TransactionPayload, TransactionResponse, TransactionUpdatePayload } from '../shared/models';

export interface TransactionFilters {
  page?: number;
  limit?: number;
  sort?: string;
  account_code?: string | string[];
  category_code?: string | string[];
  description?: string;
  operation?: string | string[];
  min_amount?: number;
  max_amount?: number;
  from_date?: string;
  to_date?: string;
}

@Injectable({ providedIn: 'root' })
export class TransactionsService {
  constructor(private readonly http: HttpClient) {}

  list(filters: TransactionFilters = {}) {
    let params = new HttpParams();
    for (const [key, value] of Object.entries(filters)) {
      const serialized = key === 'description'
        ? this.serializeDescriptionFilter(value)
        : Array.isArray(value)
          ? value.filter(Boolean).join(',')
          : value;
      if (serialized !== undefined && serialized !== null && serialized !== '') {
        params = params.set(key, String(serialized));
      }
    }
    return this.http.get<TransactionResponse>('/api/transactions', { params });
  }

  create(transaction: TransactionPayload) {
    return this.createMany([transaction]);
  }

  createMany(transactions: TransactionPayload[]) {
    return this.http
      .post<{ transactions: Transaction[] }>('/api/transactions', { transactions })
      .pipe(map((res) => res.transactions));
  }

  update(id: string, payload: TransactionUpdatePayload) {
    return this.http.patch<{ transaction: Transaction }>(`/api/transactions/${id}`, payload).pipe(map((res) => res.transaction));
  }

  updateMany(payload: BulkTransactionUpdatePayload) {
    return this.http.patch<{ updated_count: number }>('/api/transactions/bulk', payload).pipe(map((res) => res.updated_count));
  }

  delete(id: string) {
    return this.http.delete<void>(`/api/transactions/${id}`);
  }

  private serializeDescriptionFilter(value: unknown): string {
    if (typeof value !== 'string') {
      return '';
    }

    return value
      .split(/[\s,]+/)
      .map((term) => term.trim())
      .filter((term) => term.length > 0)
      .filter((term) => {
        const normalized = term.startsWith('-') ? term.slice(1) : term;
        return normalized.length >= 2;
      })
      .join(' ');
  }
}
