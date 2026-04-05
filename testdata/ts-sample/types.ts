export type UserRole = 'admin' | 'editor' | 'viewer';

export interface User {
  name: string;
  role: UserRole;
  createdAt: Date;
}

export interface Serializable {
  toJSON(): string;
}

export class BaseEntity implements Serializable {
  id: string = '';

  toJSON(): string {
    return JSON.stringify(this);
  }
}
