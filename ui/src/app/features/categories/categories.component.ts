import { Component, OnInit, computed, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';

import { CategoriesService } from '../../data/categories.service';
import { ReferenceDataService } from '../../data/reference-data.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { categoryTypeLabel } from '../../shared/labels';
import { deactivateCategoryConfirmationMessage, uiMessages } from '../../shared/messages';
import { Category, CategoryType } from '../../shared/models';
import { ToastService } from '../../shared/toast.service';

@Component({
  selector: 'app-categories',
  imports: [ReactiveFormsModule],
  template: `
    <section class="page-header">
      <div>
        <p class="eyebrow">{{ messages.page.eyebrow }}</p>
        <h1>{{ messages.page.title }}</h1>
      </div>
      <button class="primary-button" type="button" (click)="openCreate()">{{ messages.page.create }}</button>
    </section>
    <section class="panel">
      @if (loading()) {
        <p class="state-message">{{ messages.states.loading }}</p>
      } @else if (rootCategories().length === 0) {
        <p class="state-message">{{ messages.states.empty }}</p>
      } @else {
        <div class="table-wrap">
          <table>
            <thead>
              <tr>
                <th></th>
                <th>{{ messages.form.name }}</th>
                <th>{{ messages.form.type }}</th>
                <th></th>
              </tr>
            </thead>
            @for (category of rootCategories(); track category.Code) {
              <tbody>
                <tr
                  class="category-parent-table-row"
                  [class.dragging-row]="draggingCode() === category.Code"
                  [class.drag-target-row]="dropTargetCode() === category.Code"
                  draggable="true"
                  (dragstart)="onParentDragStart($event, category)"
                  (dragover)="onParentDragOver($event, category)"
                  (drop)="onParentDrop($event, category)"
                  (dragend)="onDragEnd()"
                >
                  <td class="drag-cell">
                    <button
                      class="icon-action drag-handle"
                      type="button"
                      [title]="messages.actions.dragTitle"
                      [attr.aria-label]="messages.actions.dragAria"
                      tabindex="-1"
                    >↕</button>
                  </td>
                  <td>
                    <div class="category-cell">
                      @if (childCount(category) > 0) {
                        <button
                          class="category-toggle"
                          type="button"
                          [attr.aria-label]="isExpanded(category) ? messages.actions.collapse : messages.actions.expand"
                          [attr.aria-expanded]="isExpanded(category)"
                          (click)="toggleCategory(category)"
                        >
                          {{ isExpanded(category) ? '▾' : '▸' }}
                        </button>
                      } @else {
                        <span class="category-toggle-spacer" aria-hidden="true"></span>
                      }
                      <div>
                        <strong>{{ category.Name }}</strong>
                        <p>{{ childSummary(category) }}</p>
                      </div>
                    </div>
                  </td>
                  <td>{{ categoryType(category.Type) }}</td>
                  <td class="actions-cell">
                    <button class="icon-action" type="button" [title]="messages.actions.editTitle" [attr.aria-label]="messages.actions.editAria" (click)="openEdit(category)">✎</button>
                    <button class="icon-action danger" type="button" [title]="messages.actions.deactivateTitle" [attr.aria-label]="messages.actions.deactivateAria" (click)="deactivate(category)">×</button>
                  </td>
                </tr>

                @if (childCount(category) > 0 && isExpanded(category)) {
                  @for (child of categoryChildren(category); track child.Code; let childIndex = $index) {
                    <tr
                      class="category-child-table-row"
                      [class.category-child-alt-row]="childIndex % 2 === 1"
                      [class.dragging-row]="draggingCode() === child.Code"
                      [class.drag-target-row]="dropTargetCode() === child.Code"
                      draggable="true"
                      (dragstart)="onChildDragStart($event, category, child)"
                      (dragover)="onChildDragOver($event, category, child)"
                      (drop)="onChildDrop($event, category, child)"
                      (dragend)="onDragEnd()"
                    >
                      <td class="drag-cell">
                        <button
                          class="icon-action drag-handle"
                          type="button"
                          [title]="messages.actions.dragTitle"
                          [attr.aria-label]="messages.actions.dragAria"
                          tabindex="-1"
                        >↕</button>
                      </td>
                      <td>
                        <div class="category-cell category-child-cell">
                          <span class="category-toggle-spacer category-child-spacer" aria-hidden="true"></span>
                          <div>
                            <span class="category-child-name">{{ child.Name }}</span>
                          </div>
                        </div>
                      </td>
                      <td>{{ categoryType(child.Type) }}</td>
                      <td class="actions-cell">
                        <button class="icon-action" type="button" [title]="messages.actions.editTitle" [attr.aria-label]="messages.actions.editAria" (click)="openEdit(child)">✎</button>
                        <button class="icon-action danger" type="button" [title]="messages.actions.deactivateTitle" [attr.aria-label]="messages.actions.deactivateAria" (click)="deactivate(child)">×</button>
                      </td>
                    </tr>
                  }
                }
              </tbody>
            }
          </table>
        </div>
      }
    </section>

    @if (panelOpen()) {
      <aside class="side-panel">
        <div class="panel-header">
          <h2>{{ editing() ? messages.form.editTitle : messages.form.createTitle }}</h2>
          <button class="ghost-button" type="button" (click)="closePanel()">{{ messages.actions.close }}</button>
        </div>
        <form class="form-stack" [formGroup]="form" (ngSubmit)="save()">
          <label>
            {{ messages.form.name }}
            <input formControlName="name" />
          </label>
          <label>
            {{ messages.form.type }}
            <select formControlName="type">
              <option value="INCOME">{{ messages.form.income }}</option>
              <option value="EXPENSE">{{ messages.form.expense }}</option>
            </select>
          </label>
          <label>
            {{ messages.form.description }}
            <textarea formControlName="description" rows="3"></textarea>
          </label>
          <label>
            {{ messages.form.parent }}
            <select formControlName="parent_code">
              <option value="">{{ messages.form.noParent }}</option>
              @for (category of parentOptions(); track category.Code) {
                <option [value]="category.Code">{{ category.Name }}</option>
              }
            </select>
          </label>
          <button class="primary-button" type="submit" [disabled]="form.invalid || saving()">
            {{ saving() ? messages.actions.saving : messages.actions.save }}
          </button>
        </form>
      </aside>
    }
  `,
  styles: [`
    .drag-cell {
      width: 48px;
    }

    .drag-handle {
      cursor: grab;
      min-width: 40px;
      padding: 0;
    }

    .dragging-row td {
      opacity: 0.5;
    }

    .drag-target-row td {
      background: var(--accent-soft);
    }
  `],
})
export class CategoriesComponent implements OnInit {
  private readonly fb = inject(FormBuilder);
  readonly messages = uiMessages.categories;
  readonly loading = signal(true);
  readonly saving = signal(false);
  readonly reordering = signal(false);
  readonly error = signal('');
  readonly categories = signal<Category[]>([]);
  readonly collapsedCategoryCodes = signal<Record<string, boolean>>({});
  readonly panelOpen = signal(false);
  readonly editing = signal<Category | null>(null);
  readonly draggingCode = signal<string | null>(null);
  readonly draggingParentCode = signal<string | null>(null);
  readonly dropTargetCode = signal<string | null>(null);
  readonly visibleCategories = computed(() => this.categories()
    .filter((category) => category.Type !== 'MOVEMENT')
    .map((category) => ({
      ...category,
      SubCategories: category.SubCategories?.filter((child) => child.Type !== 'MOVEMENT'),
    })));
  readonly rootCategories = computed(() => this.visibleCategories().filter((category) => !category.ParentID));
  readonly form = this.fb.nonNullable.group({
    name: ['', Validators.required],
    type: ['EXPENSE' as CategoryType, Validators.required],
    description: [''],
    parent_code: [''],
  });

  constructor(
    private readonly categoriesService: CategoriesService,
    private readonly referenceData: ReferenceDataService,
    private readonly toast: ToastService,
  ) {}

  ngOnInit(): void {
    this.load();
  }

  load(): void {
    this.loading.set(true);
    this.categoriesService.list().subscribe({
      next: (categories) => {
        this.categories.set(categories);
        this.referenceData.categories.set(categories);
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
      complete: () => this.loading.set(false),
    });
  }

  parentOptions(): Category[] {
    return this.rootCategories();
  }

  openCreate(): void {
    this.editing.set(null);
    this.setDescriptionRequired(true);
    this.form.reset({ name: '', type: 'EXPENSE', description: '', parent_code: '' });
    this.panelOpen.set(true);
  }

  openEdit(category: Category): void {
    this.editing.set(category);
    this.setDescriptionRequired(false);
    this.form.reset({
      name: category.Name,
      type: category.Type === 'MOVEMENT' ? 'EXPENSE' : category.Type,
      description: category.Description,
      parent_code: this.parentCodeFor(category),
    });
    this.panelOpen.set(true);
  }

  closePanel(): void {
    this.panelOpen.set(false);
    this.editing.set(null);
    this.setDescriptionRequired(false);
  }

  save(): void {
    if (this.form.invalid) {
      return;
    }
    this.saving.set(true);
    const value = this.form.getRawValue();
    const parentCode = value.parent_code || null;
    const editing = this.editing();
    const request = this.editing()
      ? this.categoriesService.update(editing!.Code, {
          name: value.name,
          type: value.type,
          description: value.description,
          parent_code: parentCode,
        })
      : this.categoriesService.create({
          name: value.name,
          type: value.type,
          description: value.description,
          parent_code: parentCode,
        });

    request.subscribe({
      next: () => {
        this.closePanel();
        this.reloadCategories();
      },
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.saving.set(false);
      },
      complete: () => this.saving.set(false),
    });
  }

  deactivate(category: Category): void {
    if (!window.confirm(deactivateCategoryConfirmationMessage(category.Name))) {
      return;
    }

    this.categoriesService.deactivate(category.Code).subscribe({
      next: () => this.reloadCategories(),
      error: (error) => this.toast.error(getApiErrorMessage(error)),
    });
  }

  private reloadCategories(): void {
    this.loading.set(true);
    this.referenceData.reload().subscribe({
      next: () => this.categories.set(this.referenceData.activeCategories()),
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.loading.set(false);
      },
      complete: () => this.loading.set(false),
    });
  }

  categoryType(type: CategoryType): string {
    return categoryTypeLabel(type);
  }

  toggleCategory(category: Category): void {
    this.collapsedCategoryCodes.update((current) => ({
      ...current,
      [category.Code]: !current[category.Code],
    }));
  }

  isExpanded(category: Category): boolean {
    return !this.collapsedCategoryCodes()[category.Code];
  }

  childCount(category: Category): number {
    return category.SubCategories?.length ?? 0;
  }

  categoryChildren(category: Category): Category[] {
    return category.SubCategories ?? [];
  }

  onParentDragStart(event: DragEvent, category: Category): void {
    if (this.reordering()) {
      event.preventDefault();
      return;
    }

    this.draggingCode.set(category.Code);
    this.draggingParentCode.set(null);
    this.dropTargetCode.set(category.Code);
    event.dataTransfer?.setData('text/plain', category.Code);
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move';
    }
  }

  onParentDragOver(event: DragEvent, category: Category): void {
    if (!this.draggingCode() || this.draggingParentCode() !== null || this.reordering()) {
      return;
    }

    event.preventDefault();
    this.dropTargetCode.set(category.Code);
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'move';
    }
  }

  onParentDrop(event: DragEvent, targetCategory: Category): void {
    event.preventDefault();

    const sourceCode = this.draggingCode() ?? event.dataTransfer?.getData('text/plain') ?? null;
    const parentCode = this.draggingParentCode();
    this.onDragEnd();

    if (!sourceCode || sourceCode === targetCategory.Code || parentCode !== null || this.reordering()) {
      return;
    }

    const reordered = this.moveRootCategory(sourceCode, targetCategory.Code);
    if (!reordered) {
      return;
    }

    const codes = reordered.filter((category) => category.Type !== 'MOVEMENT').map((category) => category.Code);
    this.persistReorder(reordered, { parent_code: null, codes });
  }

  onChildDragStart(event: DragEvent, parent: Category, child: Category): void {
    if (this.reordering()) {
      event.preventDefault();
      return;
    }

    this.draggingCode.set(child.Code);
    this.draggingParentCode.set(parent.Code);
    this.dropTargetCode.set(child.Code);
    event.dataTransfer?.setData('text/plain', child.Code);
    if (event.dataTransfer) {
      event.dataTransfer.effectAllowed = 'move';
    }
  }

  onChildDragOver(event: DragEvent, parent: Category, child: Category): void {
    if (!this.draggingCode() || this.draggingParentCode() !== parent.Code || this.reordering()) {
      return;
    }

    event.preventDefault();
    this.dropTargetCode.set(child.Code);
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = 'move';
    }
  }

  onChildDrop(event: DragEvent, parent: Category, targetChild: Category): void {
    event.preventDefault();

    const sourceCode = this.draggingCode() ?? event.dataTransfer?.getData('text/plain') ?? null;
    const parentCode = this.draggingParentCode();
    this.onDragEnd();

    if (!sourceCode || sourceCode === targetChild.Code || parentCode !== parent.Code || this.reordering()) {
      return;
    }

    const reordered = this.moveChildCategory(parent.Code, sourceCode, targetChild.Code);
    if (!reordered) {
      return;
    }

    const parentAfterMove = reordered.find((category) => category.Code === parent.Code);
    const codes = parentAfterMove?.SubCategories?.filter((category) => category.Type !== 'MOVEMENT').map((category) => category.Code) ?? [];
    this.persistReorder(reordered, { parent_code: parent.Code, codes });
  }

  onDragEnd(): void {
    this.draggingCode.set(null);
    this.draggingParentCode.set(null);
    this.dropTargetCode.set(null);
  }

  childSummary(category: Category): string {
    const count = this.childCount(category);
    if (count === 0) {
      return this.messages.list.noChildren;
    }

    if (count === 1) {
      return this.messages.list.childCountSingular;
    }

    return `${count} ${this.messages.list.childCountPlural}`;
  }

  private setDescriptionRequired(required: boolean): void {
    this.form.controls.description.setValidators(required ? [Validators.required] : []);
    this.form.controls.description.updateValueAndValidity({ emitEvent: false });
  }

  private persistReorder(categories: Category[], payload: { parent_code?: string | null; codes: string[] }): void {
    this.reordering.set(true);
    this.error.set('');
    this.categories.set(categories);

    this.categoriesService.reorder(payload).subscribe({
      next: () => this.reloadCategories(),
      error: (error) => {
        this.toast.error(getApiErrorMessage(error));
        this.reordering.set(false);
        this.reloadCategories();
      },
      complete: () => this.reordering.set(false),
    });
  }

  private moveRootCategory(sourceCode: string, targetCode: string): Category[] | null {
    const roots = [...this.rootCategories()];
    const sourceIndex = roots.findIndex((category) => category.Code === sourceCode);
    const targetIndex = roots.findIndex((category) => category.Code === targetCode);
    if (sourceIndex === -1 || targetIndex === -1) {
      return null;
    }

    const [movedCategory] = roots.splice(sourceIndex, 1);
    roots.splice(targetIndex, 0, movedCategory);

    return roots.map((category, index) => ({
      ...category,
      SortOrder: index + 1,
    }));
  }

  private moveChildCategory(parentCode: string, sourceCode: string, targetCode: string): Category[] | null {
    return this.rootCategories().map((category) => {
      if (category.Code !== parentCode) {
        return category;
      }

      const children = [...(category.SubCategories ?? [])];
      const sourceIndex = children.findIndex((child) => child.Code === sourceCode);
      const targetIndex = children.findIndex((child) => child.Code === targetCode);
      if (sourceIndex === -1 || targetIndex === -1) {
        return category;
      }

      const [movedChild] = children.splice(sourceIndex, 1);
      children.splice(targetIndex, 0, movedChild);

      return {
        ...category,
        SubCategories: children.map((child, index) => ({
          ...child,
          SortOrder: index + 1,
        })),
      };
    });
  }

  private parentCodeFor(category: Category): string {
    if (!category.ParentID) {
      return '';
    }

    return this.referenceData.flatCategories().find((candidate) => candidate.ID === category.ParentID)?.Code ?? '';
  }
}
