#!/usr/bin/env python3
"""Evaluate a fine-tuned model with lm-evaluation-harness and, optionally, your own exact-match set.

  python3 evaluate.py --model /models/finetuned/qwen3-8b/abc123 --tasks arc_easy,hellaswag --limit 500 \\
      --custom /data/eval.jsonl

Each task contributes its primary metric (acc_norm, acc, exact_match or f1, in that order); --custom is a JSONL
of {"prompt": ..., "expected": ...} scored by exact match on the generated continuation. score is the mean of
everything, which a GryviaModelRegistry promotionPolicy compares. Rank 0 reports score and one key per task.
"""
import argparse
import json
import re
import sys

from outputs import is_rank_zero, write_outputs

PRIMARY = ("acc_norm,none", "acc,none", "exact_match,strict-match", "exact_match,none", "f1,none")


def primary_metric(task_result):
    for k in PRIMARY:
        v = task_result.get(k)
        if isinstance(v, (int, float)):
            return float(v)
    return None


def aggregate(results, custom=None):
    """results is lm-eval's results dict ({task: {metric: value}}). Returns (score, per-task dict)."""
    per = {}
    for task, metrics in sorted((results or {}).items()):
        v = primary_metric(metrics)
        if v is not None:
            per[task] = v
    if custom is not None:
        per["custom"] = custom
    if not per:
        raise ValueError("no task produced a primary metric")
    return sum(per.values()) / len(per), per


def output_key(task):
    return re.sub(r"[^A-Za-z0-9_-]", "_", task)[:63]


def exact_match(generate, path, limit=None):
    """Share of records whose generation, stripped, starts with the expected answer."""
    hits = total = 0
    with open(path) as f:
        for line in f:
            if not line.strip():
                continue
            rec = json.loads(line)
            out = generate(rec["prompt"]).strip()
            hits += out.startswith(str(rec["expected"]).strip())
            total += 1
            if limit and total >= limit:
                break
    if total == 0:
        raise ValueError(f"{path} has no records")
    return hits / total


def hf_generator(model_path, max_new_tokens=64, cpu=False):
    import torch
    from transformers import AutoModelForCausalLM, AutoTokenizer
    tok = AutoTokenizer.from_pretrained(model_path)
    model = AutoModelForCausalLM.from_pretrained(model_path, torch_dtype=torch.float32 if cpu else torch.bfloat16,
                                                 device_map=None if cpu else "auto")

    def generate(prompt):
        ids = tok(prompt, return_tensors="pt").to(model.device)
        out = model.generate(**ids, max_new_tokens=max_new_tokens, do_sample=False)
        return tok.decode(out[0][ids["input_ids"].shape[1]:], skip_special_tokens=True)
    return generate


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--model", required=True)
    p.add_argument("--tasks", default="", help="comma-separated lm-eval tasks")
    p.add_argument("--limit", type=int, default=None, help="examples per task")
    p.add_argument("--custom", default="", help="JSONL of prompt/expected pairs")
    p.add_argument("--batch-size", default="auto")
    p.add_argument("--cpu", action="store_true")
    a = p.parse_args(argv)

    results = {}
    tasks = [t for t in a.tasks.split(",") if t]
    if tasks:
        import lm_eval
        model_args = f"pretrained={a.model},dtype={'float32' if a.cpu else 'bfloat16'}"
        out = lm_eval.simple_evaluate(model="hf", model_args=model_args, tasks=tasks, limit=a.limit,
                                      batch_size=a.batch_size, device="cpu" if a.cpu else None)
        results = out["results"]
    custom = exact_match(hf_generator(a.model, cpu=a.cpu), a.custom, a.limit) if a.custom else None
    score, per = aggregate(results, custom)
    print(json.dumps({"score": score, "tasks": per}, indent=2), flush=True)
    if is_rank_zero():
        outputs = {"score": f"{score:.6f}"}
        outputs.update({output_key(t): f"{v:.6f}" for t, v in per.items()})
        write_outputs(outputs)
    return 0


if __name__ == "__main__":
    sys.exit(main())
