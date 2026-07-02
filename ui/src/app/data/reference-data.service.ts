import { Injectable, computed, signal } from '@angular/core';
import { Observable, forkJoin, map, of, tap } from 'rxjs';

import { Account, Category, Suggestion } from '../shared/models';
import { AccountsService } from './accounts.service';
import { CategoriesService } from './categories.service';
import { SuggestionsService } from './suggestions.service';

@Injectable({ providedIn: 'root' })
export class ReferenceDataService {
  private readonly loadedSignal = signal(false);
  readonly accounts = signal<Account[]>([]);
  readonly categories = signal<Category[]>([]);
  readonly suggestions = signal<Suggestion[]>([]);
  readonly activeCategories = computed(() =>
    this.categories()
      .filter((category) => !category.DeactivatedAt)
      .map((category) => ({
        ...category,
        SubCategories: (category.SubCategories ?? []).filter((child) => !child.DeactivatedAt),
      })),
  );
  readonly flatCategories = computed(() =>
    this.categories().flatMap((category) => [category, ...(category.SubCategories ?? [])]),
  );
  readonly activeFlatCategories = computed(() =>
    this.activeCategories().flatMap((category) => [category, ...(category.SubCategories ?? [])]),
  );

  constructor(
    private readonly accountsService: AccountsService,
    private readonly categoriesService: CategoriesService,
    private readonly suggestionsService: SuggestionsService,
  ) {}

  load(): Observable<void> {
    if (this.loadedSignal()) {
      return of(void 0);
    }
    return this.reload();
  }

  reload(): Observable<void> {
    return forkJoin({
      accounts: this.accountsService.list(),
      categories: this.categoriesService.list(true),
      suggestions: this.suggestionsService.list(),
    }).pipe(
      tap(({ accounts, categories, suggestions }) => {
        this.accounts.set(accounts);
        this.categories.set(categories);
        this.suggestions.set(suggestions);
        this.loadedSignal.set(true);
      }),
      map(() => void 0),
    );
  }

  clear(): void {
    this.accounts.set([]);
    this.categories.set([]);
    this.suggestions.set([]);
    this.loadedSignal.set(false);
  }

  accountName(code: string | null | undefined): string {
    if (!code) {
      return '';
    }
    const account = this.accounts().find((candidate) => candidate.Code === code);
    if (!account) {
      return code;
    }
    return this.displayName(account, this.accounts());
  }

  categoryName(code: string | null | undefined): string {
    if (!code) {
      return '';
    }
    const category = this.flatCategories().find((candidate) => candidate.Code === code);
    if (!category) {
      return code;
    }
    return this.displayName(category, this.flatCategories());
  }

  private displayName<T extends { Code: string; Name: string; DeactivatedAt?: string | null }>(
    item: T,
    collection: T[],
  ): string {
    if (!item.DeactivatedAt) {
      return item.Name;
    }

    const normalizedName = this.normalizeName(item.Name);
    const hasNameCollision = collection.some(
      (candidate) =>
        candidate.Code !== item.Code && this.normalizeName(candidate.Name) === normalizedName,
    );
    if (!hasNameCollision) {
      return item.Name;
    }

    const formattedDate = new Intl.DateTimeFormat('pt-BR').format(new Date(item.DeactivatedAt));
    return `${item.Name} (desativada em ${formattedDate})`;
  }

  private normalizeName(name: string): string {
    return name.trim().toLocaleLowerCase('pt-BR');
  }
}
