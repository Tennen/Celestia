import type { ChangeEvent } from 'react';
import { ImagePlus, Trash2 } from 'lucide-react';
import { Button } from '../ui/button';
import { Input } from '../ui/input';
import { Textarea } from '../ui/textarea';
import type { VisionRule, VisionRuleKeyEntity, VisionRuleKeyEntityImage } from '../../lib/types';

type Props = {
  onError: (message: string) => void;
  onUpdateRule: (ruleId: string, updater: (current: VisionRule) => VisionRule) => void;
  rule: VisionRule;
};

function nextKeyEntityID(rule: VisionRule) {
  return rule.key_entities.reduce((max, item) => Math.max(max, item.id), 0) + 1;
}

function keyEntityLabel(item: VisionRuleKeyEntity) {
  const description = item.description?.trim();
  return description || `Key Entity #${item.id}`;
}

function keyEntityImageDataURL(image: VisionRuleKeyEntityImage | undefined) {
  if (!image?.base64) {
    return '';
  }
  return `data:${image.content_type || 'image/jpeg'};base64,${image.base64}`;
}

function parseImageDataURL(value: string) {
  const match = /^data:([^;,]+)?(?:;charset=[^;,]+)?;base64,(.+)$/i.exec(value);
  if (!match) {
    return null;
  }
  return {
    content_type: match[1] || undefined,
    base64: match[2],
  };
}

function readKeyEntityImageFile(file: File) {
  return new Promise<VisionRuleKeyEntityImage>((resolve, reject) => {
    const reader = new FileReader();
    reader.onerror = () => reject(new Error(`Failed to read ${file.name}`));
    reader.onload = () => {
      if (typeof reader.result !== 'string') {
        reject(new Error(`Failed to encode ${file.name}`));
        return;
      }
      const parsed = parseImageDataURL(reader.result);
      if (!parsed?.base64) {
        reject(new Error(`Failed to encode ${file.name}`));
        return;
      }
      resolve({
        base64: parsed.base64,
        content_type: file.type || parsed.content_type || 'image/jpeg',
      });
    };
    reader.readAsDataURL(file);
  });
}

export function VisionRuleKeyEntitiesEditor({ onError, onUpdateRule, rule }: Props) {
  const handleKeyEntityImageChange = async (keyEntityId: number, event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = '';
    if (!file) {
      return;
    }
    try {
      const image = await readKeyEntityImageFile(file);
      onUpdateRule(rule.id, (current) => ({
        ...current,
        key_entities: current.key_entities.map((item) => (item.id === keyEntityId ? { ...item, image } : item)),
      }));
    } catch (error) {
      onError(error instanceof Error ? error.message : 'Failed to load key entity reference image');
    }
  };

  return (
    <div className="automation-field">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="space-y-1">
          <label>Key Entities</label>
          <p className="muted">
            Define stable per-rule identities for post-event VLM matching. Each entry needs a positive ID plus at least
            one reference signal: image or description.
          </p>
        </div>
        <Button
          type="button"
          variant="secondary"
          size="sm"
          onClick={() =>
            onUpdateRule(rule.id, (current) => ({
              ...current,
              key_entities: [
                ...current.key_entities,
                {
                  id: nextKeyEntityID(current),
                },
              ],
            }))
          }
        >
          <ImagePlus className="h-4 w-4" />
          <span>Add Key Entity</span>
        </Button>
      </div>

      <div className="mt-3 space-y-3">
        {rule.key_entities.length > 0 ? (
          rule.key_entities.map((keyEntity) => {
            const imageDataURL = keyEntityImageDataURL(keyEntity.image);
            return (
              <div key={keyEntity.id} className="space-y-4 rounded-2xl border border-border/70 bg-secondary/15 p-4">
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div className="space-y-1">
                    <p className="text-sm font-semibold text-foreground">{keyEntityLabel(keyEntity)}</p>
                    <p className="muted">Stable ID {keyEntity.id}. Keep IDs unchanged so downstream statistics stay comparable.</p>
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={() =>
                      onUpdateRule(rule.id, (current) => ({
                        ...current,
                        key_entities: current.key_entities.filter((item) => item.id !== keyEntity.id),
                      }))
                    }
                  >
                    <Trash2 className="h-4 w-4" />
                    <span>Remove</span>
                  </Button>
                </div>

                <div className="automation-field-grid">
                  <div className="automation-field">
                    <label>Entity ID</label>
                    <Input
                      type="number"
                      min={1}
                      step={1}
                      value={keyEntity.id}
                      onChange={(event) =>
                        onUpdateRule(rule.id, (current) => ({
                          ...current,
                          key_entities: current.key_entities.map((item) =>
                            item.id === keyEntity.id
                              ? {
                                  ...item,
                                  id: Math.max(1, Number(event.target.value) || 1),
                                }
                              : item,
                          ),
                        }))
                      }
                    />
                  </div>
                  <div className="automation-field">
                    <label>Reference Image</label>
                    <Input type="file" accept="image/*" onChange={(event) => void handleKeyEntityImageChange(keyEntity.id, event)} />
                  </div>
                </div>

                {imageDataURL ? (
                  <div className="flex flex-wrap items-start gap-4 rounded-xl border border-dashed border-border/70 bg-background/70 p-3">
                    <img src={imageDataURL} alt={keyEntityLabel(keyEntity)} className="h-24 w-24 rounded-xl object-cover shadow-sm" />
                    <div className="flex-1 space-y-2">
                      <p className="muted">Current reference image: {keyEntity.image?.content_type || 'image/jpeg'}</p>
                      <Button
                        type="button"
                        variant="secondary"
                        size="sm"
                        onClick={() =>
                          onUpdateRule(rule.id, (current) => ({
                            ...current,
                            key_entities: current.key_entities.map((item) =>
                              item.id === keyEntity.id
                                ? {
                                    ...item,
                                    image: undefined,
                                  }
                                : item,
                            ),
                          }))
                        }
                      >
                        Clear Image
                      </Button>
                    </div>
                  </div>
                ) : null}

                <div className="automation-field">
                  <label>Description</label>
                  <Textarea
                    value={keyEntity.description ?? ''}
                    onChange={(event) =>
                      onUpdateRule(rule.id, (current) => ({
                        ...current,
                        key_entities: current.key_entities.map((item) =>
                          item.id === keyEntity.id
                            ? {
                                ...item,
                                description: event.target.value,
                              }
                            : item,
                        ),
                      }))
                    }
                    placeholder="For example: orange tabby with a blue collar"
                  />
                  <p className="muted">
                    Free-text identity hint. Use the physical traits that distinguish this individual from others in the
                    same class.
                  </p>
                </div>
              </div>
            );
          })
        ) : (
          <div className="rounded-2xl border border-dashed border-border/70 bg-background/60 p-4 text-sm text-muted-foreground">
            No key entities configured. Recognition events will still emit the detected class, but not a per-individual
            ID.
          </div>
        )}
      </div>
    </div>
  );
}
