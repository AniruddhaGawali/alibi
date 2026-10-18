# Case art

Case art lives in the Supabase bucket `alibi-assets`. To add or change art: put WebP files here
under the same paths, run `cd backend && make assets-upload`, then delete the local copies. **WebP only**
(the tool rejects anything else; convert with `cwebp -q 82 in.jpg -o out.webp`).

```
case-01/
  thumbnail.webp        16:9, ~1280x720 (dashboard card)
  scenes/               briefing scene images: 01-study.webp, ...
  suspects/             square portraits: lady-eleanor.webp, thomas.webp, arthur.webp, beatrice.webp
```
