import { computed, signal } from '@angular/core';
import { TestBed } from '@angular/core/testing';
import { of } from 'rxjs';
import { vi } from 'vitest';

import { CategoriesService } from '../../data/categories.service';
import { ReferenceDataService } from '../../data/reference-data.service';
import { Category } from '../../shared/models';
import { CategoriesComponent } from './categories.component';

describe('CategoriesComponent', () => {
  const categoriesSignal = signal<Category[]>([
    {
      ID: '1',
      UserID: 'user-1',
      ParentID: null,
      Code: 'alimentacao',
      Name: 'Alimentacao',
      Type: 'EXPENSE' as const,
      Description: '',
      CreatedAt: '2026-01-01T00:00:00Z',
      UpdatedAt: '2026-01-01T00:00:00Z',
      DeactivatedAt: null,
      SubCategories: [],
    },
  ]);

  const categoriesService = {
    list: vi.fn().mockReturnValue(of(categoriesSignal())),
    create: vi.fn().mockReturnValue(of(categoriesSignal()[0])),
    update: vi.fn().mockReturnValue(of(categoriesSignal()[0])),
    reorder: vi.fn().mockReturnValue(of(void 0)),
    deactivate: vi.fn().mockReturnValue(of(void 0)),
  };

  const referenceData = {
    categories: categoriesSignal,
    activeCategories: computed(() =>
      categoriesSignal()
        .filter((category) => !category.DeactivatedAt)
        .map((category) => ({
          ...category,
          SubCategories: (category.SubCategories ?? []).filter((child) => !child.DeactivatedAt),
        })),
    ),
    flatCategories: computed(() =>
      categoriesSignal().flatMap((category) => [category, ...(category.SubCategories ?? [])]),
    ),
    reload: vi.fn().mockReturnValue(of(void 0)),
  };

  beforeEach(async () => {
    categoriesService.list.mockClear();
    categoriesService.create.mockClear();
    categoriesService.update.mockClear();
    categoriesService.reorder.mockClear();
    categoriesService.deactivate.mockClear();
    referenceData.reload.mockClear();

    await TestBed.configureTestingModule({
      imports: [CategoriesComponent],
      providers: [
        { provide: CategoriesService, useValue: categoriesService },
        { provide: ReferenceDataService, useValue: referenceData },
      ],
    }).compileComponents();
  });

  it('creates a category without sending a code', () => {
    const fixture = TestBed.createComponent(CategoriesComponent);
    const component = fixture.componentInstance;

    component.openCreate();
    component.form.patchValue({
      name: 'Moradia',
      type: 'EXPENSE',
      description: 'despesas da casa',
      parent_code: '',
    });
    component.save();

    expect(categoriesService.create).toHaveBeenCalledWith({
      name: 'Moradia',
      type: 'EXPENSE',
      description: 'despesas da casa',
      parent_code: null,
    });
  });
});
