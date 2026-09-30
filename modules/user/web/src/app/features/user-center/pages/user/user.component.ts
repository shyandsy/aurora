import { Component, signal } from '@angular/core';
import { TranslateModule } from '@ngx-translate/core';
import { UserListComponent } from './components/user-list.component';
import { CreateEditUserComponent } from './components/create-edit-user/create-edit-user.component';
import { User } from '../../shared/models/user.dto';

@Component({
  selector: 'app-user',
  standalone: true,
  imports: [UserListComponent, CreateEditUserComponent, TranslateModule],
  templateUrl: './user.component.html',
  styleUrl: './user.component.css'
})
export class UserComponent {
  showCreateEditDialog = signal(false);
  editingUser = signal<User | null>(null);
  refreshTrigger = signal(0);

  /**
   * Handle create button click
   */
  onCreate(): void {
    this.editingUser.set(null);
    this.showCreateEditDialog.set(true);
  }

  /**
   * Handle edit event from user list
   */
  onEdit(user: User): void {
    this.editingUser.set(user);
    this.showCreateEditDialog.set(true);
  }

  /**
   * Handle user saved event
   */
  onUserSaved(): void {
    this.showCreateEditDialog.set(false);
    this.editingUser.set(null);
    // Trigger refresh of user list
    this.refreshTrigger.set(this.refreshTrigger() + 1);
  }

  /**
   * Handle dialog close
   */
  onDialogClose(): void {
    this.showCreateEditDialog.set(false);
    this.editingUser.set(null);
  }
}

