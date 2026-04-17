from .constants import PRODUCTS, METRIC_TYPES, STATUSES, LOG_LEVELS


class ValidationError(ValueError):
    pass


def validate_usage(product: str, metric_type: str, quantity: float, status: str) -> None:
    if product not in PRODUCTS:
        raise ValidationError(f"invalid product '{product}', allowed={PRODUCTS}")
    allowed = METRIC_TYPES.get(product, ())
    if metric_type not in allowed:
        raise ValidationError(
            f"invalid metric_type '{metric_type}' for product '{product}', allowed={allowed}"
        )
    if status not in STATUSES:
        raise ValidationError(f"invalid status '{status}', allowed={STATUSES}")
    if not isinstance(quantity, (int, float)) or quantity < 0:
        raise ValidationError(f"quantity must be non-negative number, got {quantity!r}")


def validate_log(level: str, message: str) -> None:
    if level not in LOG_LEVELS:
        raise ValidationError(f"invalid level '{level}', allowed={LOG_LEVELS}")
    if not isinstance(message, str) or not message.strip():
        raise ValidationError("message must be non-empty string")
