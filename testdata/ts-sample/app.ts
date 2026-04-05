import { User, UserRole } from './types';

export function createUser(name: string, role: UserRole): User {
  return { name, role, createdAt: new Date() };
}

export class UserService {
  private users: User[] = [];

  addUser(user: User): void {
    this.users.push(user);
  }

  findByRole(role: UserRole): User[] {
    return this.users.filter(u => u.role === role);
  }
}

const DEFAULT_ROLE: UserRole = 'viewer';
