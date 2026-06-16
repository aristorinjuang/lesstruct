import { ref } from 'vue'

interface ConfirmationDialogState {
  isOpen: boolean
  title: string
  message: string
  confirmButtonText: string
  action: null | ((userId: string) => void)
  userId: string
}

export function useConfirmationDialog() {
  const confirmationDialog = ref<ConfirmationDialogState>({
    isOpen: false,
    title: '',
    message: '',
    confirmButtonText: '',
    action: null,
    userId: '',
  })

  function showDialog(
    title: string,
    message: string,
    confirmButtonText: string,
    action: (userId: string) => void,
    userId: string
  ) {
    confirmationDialog.value = {
      isOpen: true,
      title,
      message,
      confirmButtonText,
      action,
      userId,
    }
  }

  function handleConfirm() {
    if (confirmationDialog.value.action) {
      confirmationDialog.value.action(confirmationDialog.value.userId)
    }
    closeDialog()
  }

  function handleCancel() {
    closeDialog()
  }

  function closeDialog() {
    confirmationDialog.value.isOpen = false
  }

  return {
    confirmationDialog,
    showDialog,
    handleConfirm,
    handleCancel,
  }
}
