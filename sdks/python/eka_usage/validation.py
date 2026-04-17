from .constants import PRODUCTS, METRIC_TYPES, STATUSES


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
