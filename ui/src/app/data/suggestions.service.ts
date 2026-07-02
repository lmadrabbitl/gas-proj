import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { map } from 'rxjs';

import { CreateSuggestionPayload, Suggestion, UpdateSuggestionPayload } from '../shared/models';

@Injectable({ providedIn: 'root' })
export class SuggestionsService {
  constructor(private readonly http: HttpClient) {}

  list() {
    return this.http
      .get<{ suggestions: Suggestion[] }>('/api/suggestions')
      .pipe(map((res) => res.suggestions));
  }

  create(payload: CreateSuggestionPayload) {
    return this.http
      .post<{ suggestion: Suggestion }>('/api/suggestions', payload)
      .pipe(map((res) => res.suggestion));
  }

  update(id: string, payload: UpdateSuggestionPayload) {
    return this.http
      .patch<{ suggestion: Suggestion }>(`/api/suggestions/${id}`, payload)
      .pipe(map((res) => res.suggestion));
  }

  delete(id: string) {
    return this.http.delete<void>(`/api/suggestions/${id}`);
  }
}
