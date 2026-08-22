export type InverseFn = () => void

type Entry = { undo: InverseFn; redo: InverseFn }

export class UndoStack {
  private stack: Entry[] = []
  private redoStack: Entry[] = []
  private recording = true

  get canUndo(): boolean {
    return this.stack.length > 0
  }
  get canRedo(): boolean {
    return this.redoStack.length > 0
  }

  push(inverse: InverseFn, forward: InverseFn) {
    if (!this.recording) return
    this.stack.push({ undo: inverse, redo: forward })
    this.redoStack.length = 0
  }

  doUndo() {
    const e = this.stack.pop()
    if (!e) return
    this.recording = false
    try {
      e.undo()
    } finally {
      this.recording = true
    }
    this.redoStack.push(e)
  }

  doRedo() {
    const e = this.redoStack.pop()
    if (!e) return
    this.recording = false
    try {
      e.redo()
    } finally {
      this.recording = true
    }
    this.stack.push(e)
  }

  clear() {
    this.stack.length = 0
    this.redoStack.length = 0
  }
}

export const undoStack = new UndoStack()
