#include "qjs.h"

JSContext *New_QJSContext(JSRuntime *rt)
{
  JSContext *ctx;
  ctx = JS_NewContext(rt);
  js_init_module_std(ctx, "qjs:std");
  js_init_module_os(ctx, "qjs:os");
  js_init_module_bjson(ctx, "qjs:bjson");
  js_set_global_objs(ctx);

  return ctx;
}

QJSRuntime *New_QJS(
    size_t memory_limit,
    size_t max_stack_size,
    size_t max_execution_time,
    size_t gc_threshold)
{
  JSRuntime *runtime;
  JSContext *ctx;

  runtime = JS_NewRuntime();

  if (!runtime)
    return NULL;

  if (memory_limit > 0)
    JS_SetMemoryLimit(runtime, memory_limit);

  JS_SetMemoryLimit(runtime, 1024 * 1024 * 100);

  if (gc_threshold > 0)
    JS_SetGCThreshold(runtime, gc_threshold);

  if (max_stack_size > 0)
    JS_SetMaxStackSize(runtime, max_stack_size);

  JS_SetMaxStackSize(runtime, 1024 * 1024 * 100);

  /* setup the the worker context */
  js_std_set_worker_new_context_func(New_QJSContext);
  /* initialize the standard objects */
  js_std_init_handlers(runtime);
  /* loader for ES6 modules */
  JS_SetModuleLoaderFunc(runtime, NULL, QJS_ModuleLoader, NULL);
  /* exit on unhandled promise rejections */
  // JS_SetHostPromiseRejectionTracker(runtime, js_std_promise_rejection_tracker, NULL);

  ctx = New_QJSContext(runtime);
  if (!ctx)
  {
    JS_FreeRuntime(runtime);
    return NULL;
  }

  QJSRuntime *qjs = (QJSRuntime *)malloc(sizeof(QJSRuntime));
  qjs->runtime = runtime;
  qjs->context = ctx;

  return qjs;
}

void QJS_FreeValue(JSContext *ctx, JSValue val)
{
  JS_FreeValue(ctx, val);
}

void QJS_Free(QJSRuntime *qjs)
{
  JS_FreeContext(qjs->context);
  JS_FreeRuntime(qjs->runtime);
  free(qjs);
}

JSValue QJS_CloneValue(JSContext *ctx, JSValue val)
{
  return JS_DupValue(ctx, val);
}

JSContext *QJS_GetContext(QJSRuntime *qjs)
{
  return qjs->context;
}

void QJS_UpdateStackTop(QJSRuntime *qjs)
{
  JS_UpdateStackTop(qjs->runtime);
}

QJSRuntime *qjs = NULL;

QJSRuntime *QJS_GetRuntime()
{
  return qjs;
}

void initialize()
{
  size_t memory_limit = 1024 * 1024 * 100;  // 100 MB
  size_t gc_threshold = 1024 * 1024 * 10;   // 10 MB
  size_t max_stack_size = 1024 * 1024 * 10; // 10 MB
  qjs = New_QJS(memory_limit, max_stack_size, 0, gc_threshold);
}
