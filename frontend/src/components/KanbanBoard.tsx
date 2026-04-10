import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  DndContext,
  DragOverlay,
  PointerSensor,
  closestCorners,
  useDroppable,
  useSensor,
  useSensors,
  type DragEndEvent,
  type DragOverEvent,
  type DragStartEvent,
  type UniqueIdentifier,
} from "@dnd-kit/core";
import { SortableContext, arrayMove, useSortable, verticalListSortingStrategy } from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";
import { Box, Card, CardContent, Chip, FormControl, InputLabel, MenuItem, Select, Stack, Typography } from "@mui/material";
import type { Task, TaskStatus } from "../types";

const statusOrder: TaskStatus[] = ["todo", "in_progress", "done"];

function labelStatus(s: TaskStatus): string {
  switch (s) {
    case "todo":
      return "To do";
    case "in_progress":
      return "In progress";
    case "done":
      return "Done";
    default:
      return s;
  }
}

function buildItemsFromTasks(tasks: Task[]): Record<TaskStatus, string[]> {
  const g: Record<TaskStatus, Task[]> = { todo: [], in_progress: [], done: [] };
  for (const t of tasks) {
    g[t.status].push(t);
  }
  for (const s of statusOrder) {
    g[s].sort((a, b) => (a.sort_order ?? 0) - (b.sort_order ?? 0));
  }
  return {
    todo: g.todo.map((t) => t.id),
    in_progress: g.in_progress.map((t) => t.id),
    done: g.done.map((t) => t.id),
  };
}

function isColumnId(id: string): id is TaskStatus {
  return id === "todo" || id === "in_progress" || id === "done";
}

function findContainer(items: Record<TaskStatus, string[]>, id: UniqueIdentifier): TaskStatus | undefined {
  const sid = String(id);
  if (isColumnId(sid)) return sid;
  for (const col of statusOrder) {
    if (items[col].includes(sid)) return col;
  }
  return undefined;
}

function ColumnDropZone({ id, children }: { id: TaskStatus; children: React.ReactNode }) {
  const { setNodeRef } = useDroppable({ id });
  return (
    <Box ref={setNodeRef} minHeight={56}>
      {children}
    </Box>
  );
}

type SortableTaskProps = {
  task: Task;
  onStatusChange: (task: Task, next: TaskStatus) => void;
  onEdit: (task: Task) => void;
};

function SortableTask({ task, onStatusChange, onEdit }: SortableTaskProps) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({ id: task.id });
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.45 : 1,
  };
  return (
    <Card ref={setNodeRef} variant="outlined" style={style} {...attributes} {...listeners} sx={{ cursor: "grab", touchAction: "none" }}>
      <CardContent sx={{ "&:last-child": { pb: 2 } }}>
        <Typography fontWeight={600}>{task.title}</Typography>
        {task.description && (
          <Typography variant="body2" color="text.secondary" sx={{ mt: 0.5 }}>
            {task.description}
          </Typography>
        )}
        <Stack direction="row" gap={1} flexWrap="wrap" sx={{ mt: 1 }} alignItems="center">
          <Chip size="small" label={task.priority} />
          <FormControl size="small" sx={{ minWidth: 140 }} onClick={(e) => e.stopPropagation()}>
            <InputLabel id={`st-${task.id}`}>Status</InputLabel>
            <Select
              labelId={`st-${task.id}`}
              label="Status"
              value={task.status}
              onChange={(e) => onStatusChange(task, e.target.value as TaskStatus)}
            >
              {statusOrder.map((s) => (
                <MenuItem key={s} value={s}>
                  {labelStatus(s)}
                </MenuItem>
              ))}
            </Select>
          </FormControl>
        </Stack>
        <Stack direction="row" gap={1} sx={{ mt: 1.5 }} onClick={(e) => e.stopPropagation()}>
          <Typography
            component="button"
            type="button"
            variant="body2"
            color="primary"
            sx={{ border: 0, background: "none", cursor: "pointer", p: 0 }}
            onClick={() => onEdit(task)}
          >
            Edit
          </Typography>
        </Stack>
      </CardContent>
    </Card>
  );
}

type KanbanBoardProps = {
  tasks: Task[];
  onReorder: (columns: Record<TaskStatus, string[]>) => Promise<void>;
  onStatusChange: (task: Task, next: TaskStatus) => void;
  onEdit: (task: Task) => void;
};

export default function KanbanBoard({ tasks, onReorder, onStatusChange, onEdit }: KanbanBoardProps) {
  const taskById = useMemo(() => {
    const m: Record<string, Task> = {};
    for (const t of tasks) m[t.id] = t;
    return m;
  }, [tasks]);

  const [items, setItems] = useState<Record<TaskStatus, string[]>>(() => buildItemsFromTasks(tasks));
  const [activeId, setActiveId] = useState<UniqueIdentifier | null>(null);
  const draggingRef = useRef(false);

  useEffect(() => {
    if (!draggingRef.current) {
      setItems(buildItemsFromTasks(tasks));
    }
  }, [tasks]);

  const sensors = useSensors(useSensor(PointerSensor, { activationConstraint: { distance: 6 } }));

  const onDragStart = useCallback((e: DragStartEvent) => {
    draggingRef.current = true;
    setActiveId(e.active.id);
  }, []);

  const onDragOver = useCallback((event: DragOverEvent) => {
    const { active, over } = event;
    const overId = over?.id;
    if (overId == null) return;

    setItems((prev) => {
      const activeContainer = findContainer(prev, active.id);
      const overContainer = findContainer(prev, overId);
      if (!activeContainer || !overContainer) return prev;
      if (activeContainer === overContainer) return prev;

      const activeItems = [...prev[activeContainer]];
      const overItems = [...prev[overContainer]];
      const activeIndex = activeItems.indexOf(String(active.id));
      if (activeIndex === -1) return prev;

      const overStr = String(overId);
      let newIndex: number;
      if (isColumnId(overStr)) {
        newIndex = overItems.length;
      } else {
        const overIndex = overItems.indexOf(overStr);
        const isBelowOverItem =
          over &&
          active.rect.current.translated &&
          active.rect.current.translated.top > over.rect.top + over.rect.height;
        const modifier = isBelowOverItem ? 1 : 0;
        newIndex = overIndex >= 0 ? overIndex + modifier : overItems.length;
      }

      const moved = activeItems[activeIndex];
      const nextActive = activeItems.filter((id) => id !== moved);
      const nextOver = [...overItems.slice(0, newIndex), moved, ...overItems.slice(newIndex)];

      return {
        ...prev,
        [activeContainer]: nextActive,
        [overContainer]: nextOver,
      };
    });
  }, []);

  const onDragEnd = useCallback(
    (event: DragEndEvent) => {
      const { active, over } = event;
      setActiveId(null);
      draggingRef.current = false;

      if (!over) {
        setItems(buildItemsFromTasks(tasks));
        return;
      }

      setItems((prev) => {
        const activeContainer = findContainer(prev, active.id);
        const overContainer = findContainer(prev, over.id);
        if (!activeContainer || !overContainer) {
          return buildItemsFromTasks(tasks);
        }

        let next: Record<TaskStatus, string[]> = prev;
        if (activeContainer === overContainer) {
          const ai = prev[activeContainer].indexOf(String(active.id));
          const oi = prev[overContainer].indexOf(String(over.id));
          if (ai !== oi && ai >= 0 && oi >= 0) {
            next = { ...prev, [activeContainer]: arrayMove(prev[activeContainer], ai, oi) };
          }
        } else {
          next = prev;
        }

        queueMicrotask(() => {
          void onReorder(next).catch(() => {
            setItems(buildItemsFromTasks(tasks));
          });
        });
        return next;
      });
    },
    [onReorder, tasks],
  );

  const onDragCancel = useCallback(() => {
    setActiveId(null);
    draggingRef.current = false;
    setItems(buildItemsFromTasks(tasks));
  }, [tasks]);

  const activeTask = activeId ? taskById[String(activeId)] : null;

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={closestCorners}
      onDragStart={onDragStart}
      onDragOver={onDragOver}
      onDragEnd={onDragEnd}
      onDragCancel={onDragCancel}
    >
      <Box
        display="grid"
        gridTemplateColumns={{ xs: "1fr", md: "repeat(3, 1fr)" }}
        gap={2}
        alignItems="start"
      >
        {statusOrder.map((col) => (
          <Stack key={col} spacing={1.5}>
            <Typography variant="subtitle2" color="text.secondary" fontWeight={700}>
              {labelStatus(col)}
            </Typography>
            <ColumnDropZone id={col}>
              <SortableContext items={items[col]} strategy={verticalListSortingStrategy}>
                <Stack spacing={1.5}>
                  {items[col].map((id) => {
                    const task = taskById[id];
                    if (!task) return null;
                    return <SortableTask key={id} task={task} onStatusChange={onStatusChange} onEdit={onEdit} />;
                  })}
                </Stack>
              </SortableContext>
            </ColumnDropZone>
          </Stack>
        ))}
      </Box>
      <DragOverlay dropAnimation={null}>
        {activeTask ? (
          <Card variant="outlined" sx={{ width: 280, boxShadow: 6 }}>
            <CardContent>
              <Typography fontWeight={600}>{activeTask.title}</Typography>
            </CardContent>
          </Card>
        ) : null}
      </DragOverlay>
    </DndContext>
  );
}
