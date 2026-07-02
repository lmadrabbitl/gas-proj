import { HttpErrorResponse } from '@angular/common/http';

import { translateApiMessage } from './api-messages';
import { ApiError } from './models';

export function getApiErrorMessage(error: unknown): string {
  if (error instanceof HttpErrorResponse) {
    const body = error.error as ApiError | undefined;
    if (typeof body?.error !== 'string') {
      const localizedMessage = translateApiMessage(body?.error?.code);
      if (localizedMessage) {
        return localizedMessage;
      }
    }

    if (error.status === 0) {
      return 'Não foi possível conectar ao serviço no momento. Verifique sua conexão e tente novamente.';
    }

    if ([500, 502, 503, 504].includes(error.status)) {
      return 'O serviço está indisponível no momento. Tente novamente em instantes.';
    }

    if (typeof body?.error === 'string') {
      return body.error;
    }
    if (body?.error?.error) {
      return body.error.error;
    }

    return 'Não foi possível concluir a solicitação agora. Tente novamente.';
  }

  return 'Erro inesperado.';
}
