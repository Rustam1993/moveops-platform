"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { Loader2, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { useEstimateWorkspace } from "@/components/estimates/estimate-workspace-context";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card";
import { Checkbox } from "@/components/ui/checkbox";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  createEstimateTask,
  deleteEstimateTask,
  getEstimateTasks,
  updateEstimateTask,
  type EstimateTask,
} from "@/lib/phase5-api";
import { getApiErrorMessage } from "@/lib/phase2-api";

export function EstimateTasksEditor() {
  const { estimate } = useEstimateWorkspace();

  const [tasks, setTasks] = useState<EstimateTask[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [newTitle, setNewTitle] = useState("");
  const [newDueDate, setNewDueDate] = useState("");
  const [editingTaskId, setEditingTaskId] = useState<string | null>(null);
  const [editingTitle, setEditingTitle] = useState("");
  const [editingDueDate, setEditingDueDate] = useState("");

  const sortedTasks = useMemo(
    () =>
      [...tasks].sort((a, b) => {
        if (a.isDone !== b.isDone) return a.isDone ? 1 : -1;
        if (a.dueAt && b.dueAt) return a.dueAt.localeCompare(b.dueAt);
        if (a.dueAt) return -1;
        if (b.dueAt) return 1;
        return b.createdAt.localeCompare(a.createdAt);
      }),
    [tasks],
  );

  const loadTasks = useCallback(async () => {
    setLoading(true);
    setLoadError(null);
    try {
      const response = await getEstimateTasks(estimate.id);
      setTasks(response.tasks);
    } catch (error) {
      setLoadError(getApiErrorMessage(error));
    } finally {
      setLoading(false);
    }
  }, [estimate.id]);

  useEffect(() => {
    void loadTasks();
  }, [loadTasks]);

  async function onCreateTask() {
    const title = newTitle.trim();
    if (!title) {
      toast.error("Task title is required");
      return;
    }

    setSaving(true);
    try {
      const response = await createEstimateTask(estimate.id, {
        title,
        dueAt: newDueDate ? new Date(`${newDueDate}T09:00`).toISOString() : undefined,
      });
      setTasks((previous) => [response.task, ...previous]);
      setNewTitle("");
      setNewDueDate("");
      toast.success("Task added");
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setSaving(false);
    }
  }

  async function onToggleDone(task: EstimateTask, isDone: boolean) {
    setSaving(true);
    try {
      const response = await updateEstimateTask(estimate.id, task.id, { isDone });
      setTasks((previous) => previous.map((entry) => (entry.id === task.id ? response.task : entry)));
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setSaving(false);
    }
  }

  function onStartEdit(task: EstimateTask) {
    setEditingTaskId(task.id);
    setEditingTitle(task.title);
    setEditingDueDate(task.dueAt ? toDateInput(task.dueAt) : "");
  }

  async function onSaveEdit(task: EstimateTask) {
    const title = editingTitle.trim();
    if (!title) {
      toast.error("Task title is required");
      return;
    }

    setSaving(true);
    try {
      const response = await updateEstimateTask(estimate.id, task.id, {
        title,
        dueAt: editingDueDate ? new Date(`${editingDueDate}T09:00`).toISOString() : undefined,
        clearDueAt: !editingDueDate,
      });
      setTasks((previous) => previous.map((entry) => (entry.id === task.id ? response.task : entry)));
      setEditingTaskId(null);
      setEditingTitle("");
      setEditingDueDate("");
      toast.success("Task updated");
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setSaving(false);
    }
  }

  async function onDeleteTask(task: EstimateTask) {
    setSaving(true);
    try {
      await deleteEstimateTask(estimate.id, task.id);
      setTasks((previous) => previous.filter((entry) => entry.id !== task.id));
      toast.success("Task deleted");
    } catch (error) {
      toast.error(getApiErrorMessage(error));
    } finally {
      setSaving(false);
    }
  }

  if (loading) {
    return (
      <Card className="border-border/70 bg-card/70">
        <CardContent className="flex h-48 items-center justify-center">
          <span className="inline-flex items-center gap-2 text-sm text-muted-foreground">
            <Loader2 className="h-4 w-4 animate-spin" />
            Loading tasks...
          </span>
        </CardContent>
      </Card>
    );
  }

  if (loadError) {
    return (
      <Card className="border-border/70 bg-card/70">
        <CardHeader>
          <CardTitle>Tasks unavailable</CardTitle>
          <CardDescription>{loadError}</CardDescription>
        </CardHeader>
        <CardContent>
          <Button variant="secondary" onClick={() => void loadTasks()}>
            <RefreshCw className="h-4 w-4" />
            Retry
          </Button>
        </CardContent>
      </Card>
    );
  }

  return (
    <div className="space-y-4">
      <Card className="border-border/70 bg-card/70">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Create task</CardTitle>
          <CardDescription>Add estimate-specific follow-up tasks and mark completion.</CardDescription>
        </CardHeader>
        <CardContent className="grid gap-3 md:grid-cols-[minmax(0,1fr)_180px_auto]">
          <div className="space-y-2">
            <Label htmlFor="new-task-title">Title</Label>
            <Input
              id="new-task-title"
              data-testid="new-task-title"
              value={newTitle}
              placeholder="Call customer to confirm details"
              onChange={(event) => setNewTitle(event.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="new-task-due-date">Due date</Label>
            <Input
              id="new-task-due-date"
              type="date"
              value={newDueDate}
              onChange={(event) => setNewDueDate(event.target.value)}
            />
          </div>
          <div className="flex items-end">
            <Button type="button" onClick={onCreateTask} disabled={saving}>
              <Plus className="h-4 w-4" />
              Add task
            </Button>
          </div>
        </CardContent>
      </Card>

      <Card className="border-border/70 bg-card/70">
        <CardHeader className="pb-3">
          <CardTitle className="text-base">Tasks List</CardTitle>
          <CardDescription>{tasks.length === 0 ? "No tasks yet." : `${tasks.length} task(s)`}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-2">
          {sortedTasks.map((task) => {
            const isEditing = editingTaskId === task.id;
            return (
              <div key={task.id} className="rounded-md border border-border/70 p-3">
                {isEditing ? (
                  <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_180px_auto]">
                    <Input value={editingTitle} onChange={(event) => setEditingTitle(event.target.value)} />
                    <Input
                      type="date"
                      value={editingDueDate}
                      onChange={(event) => setEditingDueDate(event.target.value)}
                    />
                    <div className="flex gap-2">
                      <Button size="sm" onClick={() => void onSaveEdit(task)} disabled={saving}>
                        Save
                      </Button>
                      <Button
                        size="sm"
                        variant="outline"
                        onClick={() => {
                          setEditingTaskId(null);
                          setEditingTitle("");
                          setEditingDueDate("");
                        }}
                      >
                        Cancel
                      </Button>
                    </div>
                  </div>
                ) : (
                  <div className="flex flex-wrap items-center gap-3">
                    <label className="flex min-w-0 flex-1 items-center gap-2 text-sm">
                      <Checkbox
                        checked={task.isDone}
                        onCheckedChange={(checked) => onToggleDone(task, Boolean(checked))}
                        aria-label={`Mark ${task.title} complete`}
                      />
                      <span className={task.isDone ? "text-muted-foreground line-through" : "font-medium"}>{task.title}</span>
                    </label>
                    <span className="text-xs text-muted-foreground">
                      {task.dueAt ? `Due ${formatDate(task.dueAt)}` : "No due date"}
                    </span>
                    <Button size="icon" variant="ghost" onClick={() => onStartEdit(task)} aria-label={`Edit ${task.title}`}>
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      onClick={() => void onDeleteTask(task)}
                      aria-label={`Delete ${task.title}`}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                )}
              </div>
            );
          })}
          {sortedTasks.length === 0 ? (
            <div className="rounded-md border border-dashed border-border/70 p-6 text-center text-sm text-muted-foreground">
              Add your first task to track follow-up actions for this estimate.
            </div>
          ) : null}
        </CardContent>
      </Card>
    </div>
  );
}

function formatDate(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return value;
  return new Intl.DateTimeFormat("en-US", { month: "short", day: "numeric", year: "numeric" }).format(parsed);
}

function toDateInput(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return "";
  const year = parsed.getFullYear();
  const month = String(parsed.getMonth() + 1).padStart(2, "0");
  const day = String(parsed.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}
