import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { map } from 'rxjs';

import { Category, CreateCategoryPayload, UpdateCategoryPayload } from '../shared/models';

@Injectable({ providedIn: 'root' })
export class CategoriesService {
  constructor(private readonly http: HttpClient) {}

  list(includeDeactivated = false) {
    const query = includeDeactivated ? '?include_deactivated=true' : '';
    return this.http.get<{ categories: Category[] }>(`/api/categories${query}`).pipe(map((res) => res.categories));
  }

  create(payload: CreateCategoryPayload) {
    return this.http.post<{ category: Category }>('/api/categories', payload).pipe(map((res) => res.category));
  }

  update(code: string, payload: UpdateCategoryPayload) {
    return this.http.patch<{ category: Category }>(`/api/categories/${code}`, payload).pipe(map((res) => res.category));
  }

  reorder(payload: { parent_code?: string | null; codes: string[] }) {
    return this.http.patch<void>('/api/categories/reorder', payload);
  }

  deactivate(code: string) {
    return this.http.delete<void>(`/api/categories/${code}`);
  }
}
