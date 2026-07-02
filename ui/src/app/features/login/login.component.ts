import { Component, inject, signal } from '@angular/core';
import { FormBuilder, ReactiveFormsModule, Validators } from '@angular/forms';
import { Router } from '@angular/router';
import { BadgeDollarSign, LucideAngularModule } from 'lucide-angular';

import { AuthService } from '../../core/auth.service';
import { getApiErrorMessage } from '../../shared/api-error';
import { uiMessages } from '../../shared/messages';

@Component({
  selector: 'app-login',
  imports: [ReactiveFormsModule, LucideAngularModule],
  template: `
    <main class="login-page">
      <section class="login-panel">
        <div class="login-brand">
          <span class="brand-mark" aria-hidden="true">
            <lucide-icon [img]="brandIcon" [size]="22" [strokeWidth]="2.1" aria-hidden="true" />
          </span>
          <div>
            <p class="eyebrow">{{ messages.eyebrow }}</p>
            <strong>{{ shellMessages.brandSubtitle }}</strong>
          </div>
        </div>
        <div class="auth-switch">
          <button
            class="auth-switch-button"
            type="button"
            [class.active]="mode() === 'login'"
            [disabled]="loading()"
            (click)="setMode('login')"
          >
            {{ messages.title }}
          </button>
          <button
            class="auth-switch-button"
            type="button"
            [class.active]="mode() === 'register'"
            [disabled]="loading()"
            (click)="setMode('register')"
          >
            {{ messages.registerTitle }}
          </button>
        </div>
        <h1>{{ mode() === 'login' ? messages.title : messages.registerTitle }}</h1>
        <p class="muted">{{ mode() === 'login' ? messages.subtitle : messages.registerSubtitle }}</p>

        <form class="form-stack" [formGroup]="form" (ngSubmit)="submit()">
          @if (mode() === 'register') {
            <label>
              {{ messages.name }}
              <input type="text" formControlName="name" autocomplete="name" />
            </label>
          }

          <label>
            {{ messages.email }}
            <input type="email" formControlName="email" autocomplete="email" />
          </label>

          <label>
            {{ messages.password }}
            <input
              type="password"
              formControlName="password"
              [attr.autocomplete]="mode() === 'login' ? 'current-password' : 'new-password'"
            />
          </label>

          @if (error()) {
            <p class="error-message">{{ error() }}</p>
          }

          @if (info()) {
            <p class="success-message">{{ info() }}</p>
          }

          <button class="primary-button" type="submit" [disabled]="form.invalid || loading()">
            {{
              loading()
                ? mode() === 'login'
                  ? messages.submitting
                  : messages.registering
                : mode() === 'login'
                  ? messages.submit
                  : messages.registerSubmit
            }}
          </button>
        </form>

        <div class="auth-footer">
          <p>{{ mode() === 'login' ? messages.noAccount : messages.hasAccount }}</p>
          <button class="ghost-button" type="button" [disabled]="loading()" (click)="toggleMode()">
            {{ mode() === 'login' ? messages.switchToRegister : messages.switchToLogin }}
          </button>
        </div>
      </section>
    </main>
  `,
})
export class LoginComponent {
  private readonly fb = inject(FormBuilder);
  readonly messages = uiMessages.login;
  readonly shellMessages = uiMessages.shell;
  readonly brandIcon = BadgeDollarSign;
  readonly mode = signal<'login' | 'register'>('login');
  readonly loading = signal(false);
  readonly error = signal('');
  readonly info = signal('');
  readonly form = this.fb.nonNullable.group({
    name: [''],
    email: ['', [Validators.required, Validators.email]],
    password: ['', [Validators.required, Validators.minLength(8)]],
  });

  constructor(
    private readonly auth: AuthService,
    private readonly router: Router,
  ) {}

  setMode(mode: 'login' | 'register'): void {
    this.mode.set(mode);
    this.error.set('');

    const nameControl = this.form.controls.name;
    if (mode === 'register') {
      nameControl.setValidators([Validators.required, Validators.maxLength(120)]);
    } else {
      nameControl.setValidators([]);
    }
    nameControl.updateValueAndValidity();
  }

  toggleMode(): void {
    this.info.set('');
    this.setMode(this.mode() === 'login' ? 'register' : 'login');
  }

  submit(): void {
    if (this.form.invalid) {
      return;
    }

    this.loading.set(true);
    this.error.set('');
    this.info.set('');
    const { name, email, password } = this.form.getRawValue();

    if (this.mode() === 'login') {
      this.auth.login(email, password).subscribe({
        next: () => void this.router.navigate(['/dashboard']),
        error: (error) => {
          this.error.set(getApiErrorMessage(error));
          this.loading.set(false);
        },
        complete: () => this.loading.set(false),
      });
      return;
    }

    this.auth.register({ name, email, password }).subscribe({
      next: ({ user }) => {
        this.form.patchValue({ name: '', email: user.email, password: '' });
        this.setMode('login');
        this.info.set(this.messages.registerSuccess);
      },
      error: (error) => {
        this.error.set(getApiErrorMessage(error));
        this.loading.set(false);
      },
      complete: () => this.loading.set(false),
    });
  }
}
