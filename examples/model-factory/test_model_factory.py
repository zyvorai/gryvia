import json
import os
import tempfile
import unittest

from download import download, target_dir
import evaluate
from evaluate import aggregate, exact_match, openai_generator, output_key
from finetune_lora import filter_kwargs, output_dir, validate_jsonl
from outputs import write_outputs


class OutputsTests(unittest.TestCase):
    def test_writes_json_strings(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "termination-log")
            write_outputs({"path": "/models/x", "steps": 12, "ok": True}, path)
            with open(path) as f:
                self.assertEqual(json.load(f), {"path": "/models/x", "steps": "12", "ok": "true"})

    def test_rejects_bad_keys_and_oversize(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "log")
            self.assertRaises(ValueError, write_outputs, {"bad key": "x"}, path)
            self.assertRaises(ValueError, write_outputs, {"big": "x" * 5000}, path)


class DownloadTests(unittest.TestCase):
    def test_downloads_once_per_revision(self):
        calls = []

        def fake_snapshot(repo_id, revision, local_dir, allow_patterns):
            calls.append((repo_id, revision))
            open(os.path.join(local_dir, "config.json"), "w").write("{}")

        with tempfile.TemporaryDirectory() as cache:
            p1 = download("Qwen/Qwen3-8B", "abc1234", cache, snapshot=fake_snapshot)
            p2 = download("Qwen/Qwen3-8B", "abc1234", cache, snapshot=fake_snapshot)
            self.assertEqual(p1, p2)
            self.assertEqual(p1, target_dir(cache, "Qwen/Qwen3-8B", "abc1234"))
            self.assertEqual(len(calls), 1)
            download("Qwen/Qwen3-8B", "def5678", cache, snapshot=fake_snapshot)
            self.assertEqual(len(calls), 2)


class FinetuneTests(unittest.TestCase):
    def test_output_dir_and_kwargs(self):
        self.assertEqual(output_dir("/models/ft", "qwen3-8b", "0123456789abcdef"), "/models/ft/qwen3-8b/0123456789ab")
        self.assertEqual(output_dir("/m", "x", ""), "/m/x/main")

        def new_api(output_dir, max_length=1):
            pass

        def flexible(output_dir, **kw):
            pass

        args = {"output_dir": "o", "max_length": 2, "max_seq_length": 2}
        self.assertEqual(filter_kwargs(new_api, args), {"output_dir": "o", "max_length": 2})
        self.assertEqual(filter_kwargs(flexible, args), args)

    def test_validate_jsonl(self):
        with tempfile.TemporaryDirectory() as d:
            good = os.path.join(d, "good.jsonl")
            open(good, "w").write('{"messages": [{"role": "user", "content": "hi"}]}\n\n{"text": "x"}\n')
            validate_jsonl(good)
            bad = os.path.join(d, "bad.jsonl")
            open(bad, "w").write('{"prompt": "x"}\n')
            self.assertRaises(ValueError, validate_jsonl, bad)


class EvaluateTests(unittest.TestCase):
    def test_aggregate_prefers_primary_metrics(self):
        results = {
            "hellaswag": {"acc,none": 0.5, "acc_norm,none": 0.6, "alias": "hellaswag"},
            "gsm8k": {"exact_match,strict-match": 0.3},
            "weird": {"perplexity,none": 12.0},
        }
        score, per = aggregate(results, custom=0.9)
        self.assertEqual(per, {"gsm8k": 0.3, "hellaswag": 0.6, "custom": 0.9})
        self.assertAlmostEqual(score, 0.6)
        self.assertRaises(ValueError, aggregate, {"weird": {"perplexity,none": 1.0}})

    def test_exact_match_and_keys(self):
        with tempfile.TemporaryDirectory() as d:
            path = os.path.join(d, "eval.jsonl")
            open(path, "w").write('{"prompt": "2+2=", "expected": "4"}\n{"prompt": "3+3=", "expected": 6}\n')
            answers = {"2+2=": " 4, of course", "3+3=": "7"}
            self.assertEqual(exact_match(answers.get, path), 0.5)
        self.assertEqual(output_key("mmlu/abstract algebra"), "mmlu_abstract_algebra")

    def test_endpoint_generator_and_main(self):
        seen = []

        class Resp:
            def __init__(self, body):
                self.body = body

            def __enter__(self):
                return self

            def __exit__(self, *a):
                return False

            def read(self):
                return self.body

        def opener(req, timeout):
            body = json.loads(req.data)
            seen.append((req.full_url, req.get_header("Authorization"), body))
            answer = "4" if body["prompt"] == "2+2=" else "5"
            return Resp(json.dumps({"choices": [{"text": " " + answer}]}).encode())

        os.environ["OPENAI_API_KEY"] = "sk-test"
        try:
            gen = openai_generator("http://svc:8080/", "chat", opener=opener)
            self.assertEqual(gen("2+2="), " 4")
        finally:
            del os.environ["OPENAI_API_KEY"]
        url, auth, body = seen[0]
        self.assertEqual(url, "http://svc:8080/v1/completions")
        self.assertEqual(auth, "Bearer sk-test")
        self.assertEqual((body["model"], body["temperature"]), ("chat", 0))

        with tempfile.TemporaryDirectory() as d:
            data = os.path.join(d, "eval.jsonl")
            open(data, "w").write('{"prompt": "2+2=", "expected": "4"}\n{"prompt": "3+3=", "expected": "6"}\n')
            written = {}
            orig_gen, orig_write = evaluate.openai_generator, evaluate.write_outputs
            evaluate.openai_generator = lambda e, m: openai_generator(e, m, opener=opener)
            evaluate.write_outputs = written.update
            try:
                self.assertEqual(evaluate.main(["--endpoint", "http://svc:8080", "--served-model", "chat",
                                                "--custom", data]), 0)
            finally:
                evaluate.openai_generator, evaluate.write_outputs = orig_gen, orig_write
            self.assertEqual(written["score"], "0.500000")
        with self.assertRaises(SystemExit):
            evaluate.main(["--model", "/m", "--endpoint", "http://x"])


if __name__ == "__main__":
    unittest.main()
